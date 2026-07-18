package modelcatalog

import (
	"fmt"
	"net/url"
	"strings"
)

func ParseHuggingFaceURL(rawURL string) (Source, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return Source{}, fmt.Errorf("%w: parse url: %v", ErrInvalidInput, err)
	}
	if parsed.Scheme != "https" || parsed.Host != "huggingface.co" {
		return Source{}, fmt.Errorf("%w: expected https://huggingface.co/{owner}/{repo}", ErrInvalidInput)
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(parts) != 2 {
		return Source{}, fmt.Errorf("%w: expected Hugging Face model URL with owner and repo", ErrInvalidInput)
	}
	owner, err := url.PathUnescape(parts[0])
	if err != nil {
		return Source{}, fmt.Errorf("%w: invalid owner", ErrInvalidInput)
	}
	repo, err := url.PathUnescape(parts[1])
	if err != nil {
		return Source{}, fmt.Errorf("%w: invalid repo", ErrInvalidInput)
	}
	if !safeSegment(owner) || !safeSegment(repo) {
		return Source{}, fmt.Errorf("%w: invalid Hugging Face owner or repo", ErrInvalidInput)
	}
	canonical := "https://huggingface.co/" + owner + "/" + repo
	return Source{Kind: "huggingface", Owner: owner, Repo: repo, URL: canonical}, nil
}

func safeSegment(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	return !strings.ContainsAny(value, `/\`)
}
