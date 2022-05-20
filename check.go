package resource

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/shurcooL/githubv4"
)

func LogSkipped(p *PullRequest, name string, add []string) {
	PrintLog(fmt.Sprintf("%d skipped, reason: %s", p.Number, name))
	
	for _, a := range add {
		PrintLog(a)
	}
}

func PrintLog(msg string) {
	if os.Getenv("verbose") == "true" {
		log.Print(msg)
	}
}

// Check (business logic)
func Check(request CheckRequest, manager Github) (CheckResponse, error) {
	//TODO move to input parameters
	os.Setenv("verbose", "true")

	var response CheckResponse

	// Filter out pull request if it does not have a filtered state
	filterStates := []githubv4.PullRequestState{githubv4.PullRequestStateOpen}
	if len(request.Source.States) > 0 {
		filterStates = request.Source.States
	}

	pulls, err := manager.ListPullRequests(filterStates)
	if err != nil {
		return nil, fmt.Errorf("failed to get last commits: %s", err)
	}

	disableSkipCI := request.Source.DisableCISkip

	PrintLog(fmt.Sprint("request.Version:", request.Version))
Loop:
	for _, p := range pulls {
		PrintLog("\n")
		PrintLog(fmt.Sprint("PR:", p.Number))
		PrintLog(fmt.Sprint("commit:", p.Tip.OID))

		// [ci skip]/[skip ci] in Pull request title
		if !disableSkipCI && ContainsSkipCI(p.Title) {
			LogSkipped(p, "[ci skip]/[skip ci] in Pull request title", []string{
				fmt.Sprint("disableSkipCI:", disableSkipCI),
				fmt.Sprint("p.Title:", p.Title)})
			continue
		}

		// [ci skip]/[skip ci] in Commit message
		if !disableSkipCI && ContainsSkipCI(p.Tip.Message) {
			LogSkipped(p, "ci skip]/[skip ci] in Commit message",[]string{
				fmt.Sprint("disableSkipCI:", disableSkipCI),
				fmt.Sprint("p.Tip.Message:", p.Tip.Message)})
			continue
		}

		// Filter pull request if the BaseBranch does not match the one specified in source
		if request.Source.BaseBranch != "" && p.PullRequestObject.BaseRefName != request.Source.BaseBranch {
			LogSkipped(p, "Filtr pull request if the BaseBranch does not match the one specified in source", []string{
				fmt.Sprint("request.Source.BaseBranch:", request.Source.BaseBranch),
				fmt.Sprint("p.PullRequestObject.BaseRefName:", p.PullRequestObject.BaseRefName)})
			continue
		}

		// Filter out commits that are too old.
		if request.Source.StatusContext == "" && !p.Tip.CommittedDate.Time.After(request.Version.CommittedDate) {
			LogSkipped(p, "Filter out commits that are too old.", []string{
				fmt.Sprint("request.Source.StatusContext:", request.Source.StatusContext),
				fmt.Sprint("p.Tip.CommittedDate.Time:", p.Tip.CommittedDate.Time),
				fmt.Sprint("request.Version.CommittedDate:", request.Version.CommittedDate),
			})
			continue
		}

		// Filter out commits that already have a build status
		if request.Source.StatusContext != "" && p.HasStatus {
			LogSkipped(p, "Filter out commits that already have a build status", []string{
				fmt.Sprint("request.Source.StatusContext:", request.Source.StatusContext), 
				fmt.Sprint("p.HasStatus:", p.HasStatus)})
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
				LogSkipped(p, "Filter out pull request if it does not contain at least one of the desired labels", []string{
					fmt.Sprint("request.Source.Labels:", request.Source.Labels),
					fmt.Sprint("p.Labels:", p.Labels)})
				continue Loop
			}
		}

		// Filter out forks.
		if request.Source.DisableForks && p.IsCrossRepository {
			LogSkipped(p, "Filter out forks.", []string{
				fmt.Sprint("request.Source.DisableForks:", request.Source.DisableForks),
				fmt.Sprint("p.IsCrossRepository:", p.IsCrossRepository)})
			continue
		}

		// Filter out drafts.
		if request.Source.IgnoreDrafts && p.IsDraft {
			LogSkipped(p, "Filter out drafts.", []string{
				fmt.Sprint("request.Source.IgnoreDrafts:", request.Source.IgnoreDrafts),
				fmt.Sprint("p.IsDraft:", p.IsDraft)})
			continue
		}

		// Filter pull request if it does not have the required number of approved review(s).
		if p.ApprovedReviewCount < request.Source.RequiredReviewApprovals {
			LogSkipped(p, "Filter pull request if it does not have the required number of approved review(s).", []string{
				fmt.Sprint("p.ApprovedReviewCount:", p.ApprovedReviewCount ),
				fmt.Sprint("request.Source.RequiredReviewApprovals:", request.Source.RequiredReviewApprovals)})
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
				LogSkipped(p, "Skip version if no files match the specified paths.", []string{
					fmt.Sprint("request.Source.Paths:", request.Source.Paths),
					fmt.Sprint("wanted:", wanted)})
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
				LogSkipped(p, "Skip version if all files are ignored.", []string{
					fmt.Sprint("request.Source.IgnorePaths:", request.Source.IgnorePaths),
					fmt.Sprint("wanted:", wanted)})
				continue Loop
			}
		}
		response = append(response, NewVersion(p))

		PrintLog("not skipped")
	}

	// Sort the commits by date
	sort.Sort(response)

	PrintLog(fmt.Sprint("response length before filter:", len(response)))

	// If there are no new but an old version = return the old
	if len(response) == 0 && request.Version.PR != "" {
		response = append(response, request.Version)
	}
	// If there are new versions and no previous = return just the latest
	if len(response) != 0 && request.Version.PR == "" {
		response = CheckResponse{response[len(response)-1]}
	}

	PrintLog(fmt.Sprint("response length after filter:", len(response)))

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

// CheckRequest ...
type CheckRequest struct {
	Source  Source  `json:"source"`
	Version Version `json:"version"`
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
