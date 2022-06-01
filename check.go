package resource

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"strconv"

	"github.com/shurcooL/githubv4"
)

// Check (business logic)
func Check(request CheckRequest, manager Github) (CheckResponse, error) {
	var response CheckResponse

	// Filter out pull request if it does not have a filtered state
	filterStates := []githubv4.PullRequestState{githubv4.PullRequestStateOpen}
	if len(request.Source.States) > 0 {
		filterStates = request.Source.States
	}

	uncheckedParameters := request.Parameters
	var checkedParameters Parameters
	if &uncheckedParameters != nil {
		checkedParameters = SetPaginationParameters(uncheckedParameters)
	}

	pulls, err := manager.ListPullRequests(filterStates, checkedParameters)

	if err != nil {
		return nil, fmt.Errorf("failed to get last commits: %s", err)
	}

	disableSkipCI := request.Source.DisableCISkip

Loop:
	for _, p := range pulls {
		// [ci skip]/[skip ci] in Pull request title
		if !disableSkipCI && ContainsSkipCI(p.Title) {
			continue
		}

		// [ci skip]/[skip ci] in Commit message
		if !disableSkipCI && ContainsSkipCI(p.Tip.Message) {
			continue
		}

		// Filter pull request if the BaseBranch does not match the one specified in source
		if request.Source.BaseBranch != "" && p.PullRequestObject.BaseRefName != request.Source.BaseBranch {
			continue
		}

		// Filter out commits that are too old.
		if request.Source.StatusContext == "" && !p.Tip.CommittedDate.Time.After(request.Version.CommittedDate) {
			continue
		}

		// Filter out commits that already have a build status
		if request.Source.StatusContext != "" && p.HasStatus {
			continue
		}

		// Filter out pull request if it does not contain at least one of the desired labels
		if len(request.Source.Labels) > 0 {
			labelFound := false

		LabelLoop:
			for _, wantedLabel := range request.Source.Labels {
				for _, targetLabel := range p.Labels {
					if targetLabel.Name == wantedLabel {
						labelFound = true
						break LabelLoop
					}
				}
			}

			if !labelFound {
				continue Loop
			}
		}

		// Filter out forks.
		if request.Source.DisableForks && p.IsCrossRepository {
			continue
		}

		// Filter out drafts.
		if request.Source.IgnoreDrafts && p.IsDraft {
			continue
		}

		// Filter pull request if it does not have the required number of approved review(s).
		if p.ApprovedReviewCount < request.Source.RequiredReviewApprovals {
			continue
		}

		// Fetch files once if paths/ignore_paths are specified.
		var files []string

		if len(request.Source.Paths) > 0 || len(request.Source.IgnorePaths) > 0 {
			files, err = manager.ListModifiedFiles(p.Number)
			if err != nil {
				return nil, fmt.Errorf("failed to list modified files: %s", err)
			}
		}

		// Skip version if no files match the specified paths.
		if len(request.Source.Paths) > 0 {
			var wanted []string
			for _, pattern := range request.Source.Paths {
				w, err := FilterPath(files, pattern)
				if err != nil {
					return nil, fmt.Errorf("path match failed: %s", err)
				}
				wanted = append(wanted, w...)
			}
			if len(wanted) == 0 {
				continue Loop
			}
		}

		// Skip version if all files are ignored.
		if len(request.Source.IgnorePaths) > 0 {
			wanted := files
			for _, pattern := range request.Source.IgnorePaths {
				wanted, err = FilterIgnorePath(wanted, pattern)
				if err != nil {
					return nil, fmt.Errorf("ignore path match failed: %s", err)
				}
			}
			if len(wanted) == 0 {
				continue Loop
			}
		}
		response = append(response, NewVersion(p))
	}

	// Sort the commits by date
	sort.Sort(response)

	// If there are no new but an old version = return the old
	if len(response) == 0 && request.Version.PR != "" {
		response = append(response, request.Version)
	}
	// If there are new versions and no previous = return just the latest
	if len(response) != 0 && request.Version.PR == "" {
		response = CheckResponse{response[len(response)-1]}
	}
	return response, nil
}

// ContainsSkipCI returns true if a string contains [ci skip] or [skip ci].
func ContainsSkipCI(s string) bool {
	re := regexp.MustCompile("(?i)\\[(ci skip|skip ci)\\]")
	return re.MatchString(s)
}

// FilterIgnorePath ...
func FilterIgnorePath(files []string, pattern string) ([]string, error) {
	var out []string
	for _, file := range files {
		match, err := filepath.Match(pattern, file)
		if err != nil {
			return nil, err
		}
		if !match && !IsInsidePath(pattern, file) {
			out = append(out, file)
		}
	}
	return out, nil
}

// FilterPath ...
func FilterPath(files []string, pattern string) ([]string, error) {
	var out []string
	for _, file := range files {
		match, err := filepath.Match(pattern, file)
		if err != nil {
			return nil, err
		}
		if match || IsInsidePath(pattern, file) {
			out = append(out, file)
		}
	}
	return out, nil
}

// IsInsidePath checks whether the child path is inside the parent path.
//
// /foo/bar is inside /foo, but /foobar is not inside /foo.
// /foo is inside /foo, but /foo is not inside /foo/
func IsInsidePath(parent, child string) bool {
	if parent == child {
		return true
	}

	// we add a trailing slash so that we only get prefix matches on a
	// directory separator
	parentWithTrailingSlash := parent
	if !strings.HasSuffix(parentWithTrailingSlash, string(filepath.Separator)) {
		parentWithTrailingSlash += string(filepath.Separator)
	}

	return strings.HasPrefix(child, parentWithTrailingSlash)
}

func SetPaginationParameters(p Parameters) Parameters {
	var outputParameters Parameters

	if p.PageSize == 0 {
		outputParameters.PageSize = 50
		log.Printf("No page_size specified, using default value 50")
	} else if p.PageSize > 200 {
		outputParameters.PageSize = 200
		log.Printf("Max page_size exceeded, using max value 200")
	} else {
		outputParameters.PageSize = p.PageSize
		log.Printf("running with specified per_page: %s", strconv.Itoa(p.PageSize))
	}

	if p.MaxPRs == 0 {
		outputParameters.MaxPRs = 200
		log.Printf("No max_prs value specified, using default value 200")
	} else if p.MaxPRs > 500 {
		outputParameters.MaxPRs = 500
		log.Printf("max max_prs value exceeded, using max value 500")
	} else {
		log.Printf("running with specified max_prs: %s", strconv.Itoa(p.MaxPRs))
		outputParameters.MaxPRs = p.MaxPRs
	}

	if p.MaxRetries == 0 {
		outputParameters.MaxRetries = 4
		log.Printf("No max_retries value specified, using default value 4")
	} else if p.MaxRetries > 10 {
		outputParameters.MaxRetries = 10
		log.Printf("max max_retries value exceeded, using max value 10")
	} else {
		log.Printf("running with specified max_retries: %s", strconv.Itoa(p.MaxRetries))
	}

	if p.DelayBetweenPages == 0 {
		outputParameters.DelayBetweenPages = 500
		log.Printf("No delay_between_pages value specified, using default value 500ms")
	} else if p.DelayBetweenPages > 10000 {
		outputParameters.DelayBetweenPages = 10000
		log.Printf("max delay_between_pages value exceeded, using max value 10,000ms")
	} else {
		outputParameters.DelayBetweenPages = p.DelayBetweenPages
		log.Printf("running with specified delay_between_pages: %sms", strconv.Itoa(p.DelayBetweenPages))
	}

	if p.SortField == "" {
		outputParameters.SortField = "UPDATED_AT"
		log.Printf("No sort_field specified, using default value 'UPDATED_AT'")
	} else {
		outputParameters.SortField = p.SortField
		log.Printf("running with specified sort_direction: %s", p.SortField)
	}

	switch p.SortDirection {
	case "DESC":
		outputParameters.SortDirection = "DESC"
		log.Printf("running with specified sort_direction: 'DESC'")
	case "ASC":
		outputParameters.SortDirection = "ASC"
		log.Printf("running with specified sort_direction: 'ASC'")
	case "":
		outputParameters.SortDirection = "DESC"
		log.Printf("No sort_direction specified, using default value 'DESC'")
	default:
		outputParameters.SortDirection = "DESC"
		log.Printf("sort_direction %s not valid, using default value 'DESC'", p.SortDirection)
	}

	return outputParameters

}

// CheckRequest ...
type CheckRequest struct {
	Source     Source     `json:"source"`
	Version    Version    `json:"version"`
	Parameters Parameters `json:"parameters"`
}

// CheckResponse ...
type CheckResponse []Version

func (r CheckResponse) Len() int {
	return len(r)
}

func (r CheckResponse) Less(i, j int) bool {
	return r[j].CommittedDate.After(r[i].CommittedDate)
}

func (r CheckResponse) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}
