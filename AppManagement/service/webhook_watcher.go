package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/webhook"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"go.uber.org/zap"
)

// crashNotifyCooldown bounds how often a single compose service can send a
// container_crash notification - a container stuck in a restart loop dies
// repeatedly in quick succession, and without this a user gets one message
// per restart (confirmed live: reported as "container crashed a lot").
// Crashes during the cooldown are still counted, just folded into the next
// notification instead of each firing its own.
const crashNotifyCooldown = 15 * time.Minute

type crashWatcherState struct {
	mu         sync.Mutex
	lastSentAt map[string]time.Time
	countSince map[string]int
}

var crashState = &crashWatcherState{
	lastSentAt: map[string]time.Time{},
	countSince: map[string]int{},
}

// expectedStopWindow is how long after CasaOS itself stops, restarts,
// uninstalls, or recreates (an update or a settings apply) a compose app's
// containers the resulting "die" event(s) are treated as expected rather
// than a crash. Long enough to cover a slow stop/pull/recreate, short
// enough that the same app dying again later for real reasons still gets
// reported.
const expectedStopWindow = 5 * time.Minute

type expectedStopState struct {
	mu    sync.Mutex
	until map[string]time.Time // compose project (app) name -> expires
}

var expectedStops = &expectedStopState{until: map[string]time.Time{}}

// ExpectAppStop marks appName (a compose app/project name) as about to be
// intentionally stopped, restarted, uninstalled, or recreated by CasaOS
// itself, so the crash watcher doesn't mistake the container "die" event(s)
// that causes for a crash. Call this right before triggering the action.
func ExpectAppStop(appName string) {
	expectedStops.mu.Lock()
	defer expectedStops.mu.Unlock()
	expectedStops.until[appName] = time.Now().Add(expectedStopWindow)
}

func (s *expectedStopState) isExpected(appName string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	until, ok := s.until[appName]
	if !ok {
		return false
	}
	if time.Now().After(until) {
		delete(s.until, appName)
		return false
	}
	return true
}

// recordAndShouldNotify records one crash for key and reports whether the
// cooldown has elapsed since the last notification for it. count is the
// number of crashes (including this one) since the last notification was
// actually sent, reset to zero whenever notify is true.
func (s *crashWatcherState) recordAndShouldNotify(key string) (count int, notify bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.countSince[key]++
	if last, seen := s.lastSentAt[key]; seen && time.Since(last) < crashNotifyCooldown {
		return s.countSince[key], false
	}
	count = s.countSince[key]
	s.countSince[key] = 0
	s.lastSentAt[key] = time.Now()
	return count, true
}

// StartCrashWatcher watches the Docker event stream for containers that stop
// unexpectedly (a non-zero exit code, as opposed to a graceful stop a user or
// CasaOS itself initiated) and fires a container_crash webhook. Only
// containers belonging to a known Docker Compose project are considered -
// Compose already labels every container it manages
// (com.docker.compose.project/service), and Docker's event stream includes a
// container's labels directly in Actor.Attributes, so no extra lookup is
// needed to scope this to "one of my apps" vs. some unrelated container.
//
// Runs until ctx is cancelled. Reconnects on stream errors rather than giving
// up, since a Docker daemon restart or brief API hiccup shouldn't
// permanently disable crash notifications.
func StartCrashWatcher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		watchOnce(ctx)
		time.Sleep(5 * time.Second)
	}
}

func watchOnce(ctx context.Context) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		logger.Error("webhook watcher: failed to create docker client", zap.Error(err))
		return
	}
	defer cli.Close()

	eventFilters := filters.NewArgs(
		filters.Arg("type", "container"),
		filters.Arg("event", "die"),
	)

	messages, errs := cli.Events(ctx, types.EventsOptions{Filters: eventFilters})
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errs:
			if err != nil {
				logger.Error("webhook watcher: event stream error, reconnecting", zap.Error(err))
			}
			return
		case msg := <-messages:
			handleDieEvent(msg)
		}
	}
}

func handleDieEvent(msg events.Message) {
	project := msg.Actor.Attributes["com.docker.compose.project"]
	if project == "" {
		// Not a container Docker Compose (and therefore CasaOS) manages -
		// could be anything else running on the host.
		return
	}

	if expectedStops.isExpected(project) {
		// CasaOS itself just stopped/restarted/uninstalled/recreated this
		// app - a non-zero exit here (SIGTERM/SIGKILL during a forced stop
		// is common and normal) isn't a crash.
		return
	}

	exitCode := msg.Actor.Attributes["exitCode"]
	if exitCode == "" || exitCode == "0" {
		// A clean exit is almost always a user- or CasaOS-initiated stop,
		// not a crash - this heuristic isn't perfect (a container could
		// legitimately exit 0 in a crash loop, or a forceful kill could
		// still show non-zero even when intentional), but it's the standard
		// signal tools like this use to tell the two apart.
		return
	}

	serviceName := msg.Actor.Attributes["com.docker.compose.service"]
	containerName := msg.Actor.Attributes["name"]

	count, notify := crashState.recordAndShouldNotify(project + "/" + serviceName)
	if !notify {
		return
	}

	message := fmt.Sprintf("%s (service %s in %s) exited with code %s", containerName, serviceName, project, exitCode)
	if count > 1 {
		message = fmt.Sprintf("%s - crashed %d times in the last %d minutes", message, count, int(crashNotifyCooldown.Minutes()))
	}

	webhook.Send(
		"container_crash",
		"Container crashed",
		message,
		map[string]string{
			"app":         project,
			"service":     serviceName,
			"exit_code":   exitCode,
			"crash_count": fmt.Sprintf("%d", count),
		},
	)
}
