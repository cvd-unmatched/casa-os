package service

import (
	"context"
	"fmt"
	"time"

	"github.com/IceWhaleTech/CasaOS-AppManagement/pkg/webhook"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"go.uber.org/zap"
)

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

	webhook.Send(
		"container_crash",
		"Container crashed",
		fmt.Sprintf("%s (service %s in %s) exited with code %s", containerName, serviceName, project, exitCode),
		map[string]string{
			"app":       project,
			"service":   serviceName,
			"exit_code": exitCode,
		},
	)
}
