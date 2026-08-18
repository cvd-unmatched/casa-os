/*
credit: https://github.com/containrrr/watchtower
*/
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	url2 "net/url"
	"regexp"
	"strings"

	ref "github.com/docker/distribution/reference"
)

// BuildTagsURL mirrors BuildManifestURL (manifest.go), but targets the
// /v2/<name>/tags/list endpoint instead of /v2/<name>/manifests/<tag> - the
// tag/digest portion of imageName itself is ignored, only the repository
// path matters.
func BuildTagsURL(imageName string) (string, error) {
	normalizedName, err := ref.ParseNormalizedNamed(imageName)
	if err != nil {
		return "", err
	}

	host, err := NormalizeRegistry(normalizedName.String())
	img, _ := ExtractImageAndTag(strings.TrimPrefix(imageName, host+"/"))

	if err != nil {
		return "", err
	}
	img = GetScopeFromImageName(img, host)

	if !strings.Contains(img, "/") {
		img = "library/" + img
	}
	url := url2.URL{
		Scheme: "https",
		Host:   host,
		Path:   fmt.Sprintf("/v2/%s/tags/list", img),
	}
	return url.String(), nil
}

type tagsListResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// maxTagsPages guards against a misbehaving registry looping forever via
// Link-header pagination - generous for personal images with a few dozen
// tags (at typical page sizes this covers on the order of 1000+ tags).
const maxTagsPages = 20

var linkNextRe = regexp.MustCompile(`<(.*?)>;\s*rel="next"`)

// GetTags returns every tag the registry reports for imageName's
// repository, following OCI Link-header pagination. Pure read - does not
// touch GetManifest/GetDigest/CompareDigest.
func GetTags(ctx context.Context, imageName string) ([]string, error) {
	opts, err := GetPullOptions(imageName)
	if err != nil {
		return nil, err
	}
	registryAuth := TransformAuth(opts.RegistryAuth)

	challenge, err := GetChallenge(imageName)
	if err != nil {
		return nil, err
	}

	token, err := GetToken(challenge, registryAuth, imageName)
	if err != nil {
		return nil, err
	}

	url, err := BuildTagsURL(imageName)
	if err != nil {
		return nil, err
	}

	var all []string
	for page := 0; url != "" && page < maxTagsPages; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		addDefaultHeaders(&req.Header, token)

		res, err := httpClient().Do(req)
		if err != nil {
			return nil, err
		}

		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			return nil, fmt.Errorf("registry responded to tags list with %q", res.Status)
		}

		var body tagsListResponse
		decodeErr := json.NewDecoder(res.Body).Decode(&body)
		res.Body.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}
		all = append(all, body.Tags...)

		url = nextTagsPageURL(res.Header.Get("Link"), url)
	}

	return all, nil
}

// nextTagsPageURL resolves the OCI Link response header (rel="next") for
// pagination. Registries commonly return a relative URL there, so it's
// resolved against the same scheme+host as the request that produced it.
func nextTagsPageURL(linkHeader, currentURL string) string {
	if linkHeader == "" {
		return ""
	}
	match := linkNextRe.FindStringSubmatch(linkHeader)
	if match == nil {
		return ""
	}
	next := match[1]

	base, err := url2.Parse(currentURL)
	if err != nil {
		return ""
	}
	resolved, err := url2.Parse(next)
	if err != nil {
		return ""
	}
	return base.ResolveReference(resolved).String()
}
