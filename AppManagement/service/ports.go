package service

import (
	"context"
	"sort"
	"strconv"
	"strings"
)

// PortUsage is one published (host-exposed) port belonging to one service
// of one installed compose app.
type PortUsage struct {
	AppName     string `json:"app_name"`
	DisplayName string `json:"display_name"`
	ServiceName string `json:"service_name"`
	Published   string `json:"published"` // host port, or a range like "8080-8090"
	Target      uint32 `json:"target"`    // container port
	Protocol    string `json:"protocol"`
}

// ListPortUsage returns every published port across every installed
// compose app, for a "what's using which port" overview - a service with
// no host publish (container-internal only, reached through another
// service instead) is skipped, since it's not something that could
// conflict with anything else on the host.
func ListPortUsage(ctx context.Context) ([]PortUsage, error) {
	composeApps, err := MyService.Compose().List(ctx)
	if err != nil {
		return nil, err
	}

	var result []PortUsage
	for _, composeApp := range composeApps {
		displayName := composeAppDisplayName(composeApp)
		for _, svc := range composeApp.Services {
			for _, p := range svc.Ports {
				if p.Published == "" {
					continue
				}
				result = append(result, PortUsage{
					AppName:     composeApp.Name,
					DisplayName: displayName,
					ServiceName: svc.Name,
					Published:   p.Published,
					Target:      p.Target,
					Protocol:    p.Protocol,
				})
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		pi, pj := firstPortNumber(result[i].Published), firstPortNumber(result[j].Published)
		if pi != pj {
			return pi < pj
		}
		return result[i].Published < result[j].Published
	})

	return result, nil
}

// firstPortNumber parses the leading number out of a published port value
// ("8080", or the start of a range like "8080-8090") for numeric sorting -
// a plain string sort would put "9000" before "80", which isn't what a
// human scanning a port list expects. Falls back to 0 (sorts first) for
// anything unparseable, which just means it sorts by the string comparison
// that follows instead.
func firstPortNumber(published string) int {
	digits := published
	if idx := strings.IndexAny(published, "-/"); idx != -1 {
		digits = published[:idx]
	}
	n, err := strconv.Atoi(strings.TrimSpace(digits))
	if err != nil {
		return 0
	}
	return n
}
