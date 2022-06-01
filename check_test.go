package resource_test

import (
	"testing"

	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	resource "github.com/telia-oss/github-pr-resource"
	"github.com/telia-oss/github-pr-resource/fakes"
)

var (
	testPullRequests = []*resource.PullRequest{
		createTestPR(1, "master", true, false, 0, nil, false, githubv4.PullRequestStateOpen, false),
		createTestPR(2, "master", false, false, 0, nil, false, githubv4.PullRequestStateOpen, true),
		createTestPR(3, "master", false, false, 0, nil, true, githubv4.PullRequestStateOpen, false),
		createTestPR(4, "master", false, false, 0, nil, false, githubv4.PullRequestStateOpen, true),
		createTestPR(5, "master", false, true, 0, nil, false, githubv4.PullRequestStateOpen, false),
		createTestPR(6, "master", false, false, 0, nil, false, githubv4.PullRequestStateOpen, false),
		createTestPR(7, "develop", false, false, 0, []string{"enhancement"}, false, githubv4.PullRequestStateOpen, true),
		createTestPR(8, "master", false, false, 1, []string{"wontfix"}, false, githubv4.PullRequestStateOpen, true),
		createTestPR(9, "master", false, false, 0, nil, false, githubv4.PullRequestStateOpen, false),
		createTestPR(10, "master", false, false, 0, nil, false, githubv4.PullRequestStateClosed, false),
		createTestPR(11, "master", false, false, 0, nil, false, githubv4.PullRequestStateMerged, false),
		createTestPR(12, "master", false, false, 0, nil, false, githubv4.PullRequestStateOpen, false),
	}
)

func TestCheck(t *testing.T) {
	tests := []struct {
		description  string
		source       resource.Source
		parameters   resource.Parameters
		version      resource.Version
		files        [][]string
		pullRequests []*resource.PullRequest
		expected     resource.CheckResponse
	}{
		{
			description: "check returns the latest version if there is no previous",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
			},
			version:      resource.Version{},
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[1]),
			},
		},

		{
			description: "check returns the previous version when its still latest",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
			},
			version:      resource.NewVersion(testPullRequests[1]),
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[1]),
			},
		},

		{
			description: "check returns all new versions since the last",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
			},
			version:      resource.NewVersion(testPullRequests[3]),
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[2]),
				resource.NewVersion(testPullRequests[1]),
			},
		},

		{
			description: "check will only return versions that match the specified paths",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
				Paths:       []string{"terraform/*/*.tf", "terraform/*/*/*.tf"},
			},
			version:      resource.NewVersion(testPullRequests[3]),
			pullRequests: testPullRequests,
			files: [][]string{
				{"README.md", "travis.yml"},
				{"terraform/modules/ecs/main.tf", "README.md"},
				{"terraform/modules/variables.tf", "travis.yml"},
			},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[2]),
			},
		},

		{
			description: "check will skip versions which only match the ignore paths",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
				IgnorePaths: []string{"*.md", "*.yml"},
			},
			version:      resource.NewVersion(testPullRequests[3]),
			pullRequests: testPullRequests,
			files: [][]string{
				{"README.md", "travis.yml"},
				{"terraform/modules/ecs/main.tf", "README.md"},
				{"terraform/modules/variables.tf", "travis.yml"},
			},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[2]),
			},
		},

		{
			description: "check correctly ignores [skip ci] when specified",
			source: resource.Source{
				Repository:    "itsdalmo/test-repository",
				AccessToken:   "oauthtoken",
				DisableCISkip: true,
			},
			version:      resource.NewVersion(testPullRequests[1]),
			pullRequests: testPullRequests,
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[0]),
			},
		},

		{
			description: "check correctly ignores drafts when drafts are ignored",
			source: resource.Source{
				Repository:   "itsdalmo/test-repository",
				AccessToken:  "oauthtoken",
				IgnoreDrafts: true,
			},
			version:      resource.NewVersion(testPullRequests[3]),
			pullRequests: testPullRequests,
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[1]),
			},
		},

		{
			description: "check does not ignore drafts when drafts are not ignored",
			source: resource.Source{
				Repository:   "itsdalmo/test-repository",
				AccessToken:  "oauthtoken",
				IgnoreDrafts: false,
			},
			version:      resource.NewVersion(testPullRequests[3]),
			pullRequests: testPullRequests,
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[2]),
				resource.NewVersion(testPullRequests[1]),
			},
		},

		{
			description: "check correctly ignores cross repo pull requests",
			source: resource.Source{
				Repository:   "itsdalmo/test-repository",
				AccessToken:  "oauthtoken",
				DisableForks: true,
			},
			version:      resource.NewVersion(testPullRequests[5]),
			pullRequests: testPullRequests,
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[3]),
				resource.NewVersion(testPullRequests[2]),
				resource.NewVersion(testPullRequests[1]),
			},
		},

		{
			description: "check supports specifying base branch",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
				BaseBranch:  "develop",
			},
			version:      resource.Version{},
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[6]),
			},
		},

		{
			description: "check correctly ignores PRs with no approved reviews when specified",
			source: resource.Source{
				Repository:              "itsdalmo/test-repository",
				AccessToken:             "oauthtoken",
				RequiredReviewApprovals: 1,
			},
			version:      resource.NewVersion(testPullRequests[8]),
			pullRequests: testPullRequests,
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[7]),
			},
		},

		{
			description: "check returns latest version from a PR with at least one of the desired labels on it",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
				Labels:      []string{"enhancement"},
			},
			version:      resource.Version{},
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[6]),
			},
		},

		{
			description: "check returns latest version from a PR with a single state filter",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
				States:      []githubv4.PullRequestState{githubv4.PullRequestStateClosed},
			},
			version:      resource.Version{},
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[9]),
			},
		},

		{
			description: "check filters out versions from a PR which do not match the state filter",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
				States:      []githubv4.PullRequestState{githubv4.PullRequestStateOpen},
			},
			version:      resource.Version{},
			pullRequests: testPullRequests[9:11],
			files:        [][]string{},
			expected:     resource.CheckResponse(nil),
		},

		{
			description: "check returns versions from a PR with multiple state filters",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
				States:      []githubv4.PullRequestState{githubv4.PullRequestStateClosed, githubv4.PullRequestStateMerged},
			},
			version:      resource.NewVersion(testPullRequests[11]),
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[9]),
				resource.NewVersion(testPullRequests[10]),
			},
		},

		{
			description: "check returns versions with no status",
			source: resource.Source{
				Repository:    "itsdalmo/test-repository",
				AccessToken:   "oauthtoken",
				StatusContext: "some-status",
			},
			version:      resource.NewVersion(testPullRequests[11]),
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[11]),
				resource.NewVersion(testPullRequests[8]),
				resource.NewVersion(testPullRequests[5]),
				resource.NewVersion(testPullRequests[4]),
				resource.NewVersion(testPullRequests[2]),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			github := new(fakes.FakeGithub)
			pullRequests := []*resource.PullRequest{}
			filterStates := []githubv4.PullRequestState{githubv4.PullRequestStateOpen}
			if len(tc.source.States) > 0 {
				filterStates = tc.source.States
			}
			for i := range tc.pullRequests {
				for j := range filterStates {
					if filterStates[j] == tc.pullRequests[i].PullRequestObject.State {
						pullRequests = append(pullRequests, tc.pullRequests[i])
						break
					}
				}
			}
			github.ListPullRequestsReturns(pullRequests, nil)

			for i, file := range tc.files {
				github.ListModifiedFilesReturnsOnCall(i, file, nil)
			}

			input := resource.CheckRequest{Source: tc.source, Version: tc.version, Parameters: tc.parameters}
			output, err := resource.Check(input, github)

			if assert.NoError(t, err) {
				assert.Equal(t, tc.expected, output)
			}
			assert.Equal(t, 1, github.ListPullRequestsCallCount())

		})
	}
}

func TestContainsSkipCI(t *testing.T) {
	tests := []struct {
		description string
		message     string
		want        bool
	}{
		{
			description: "does not just match any symbol in the regexp",
			message:     "(",
			want:        false,
		},
		{
			description: "does not match when it should not",
			message:     "test",
			want:        false,
		},
		{
			description: "matches [ci skip]",
			message:     "[ci skip]",
			want:        true,
		},
		{
			description: "matches [skip ci]",
			message:     "[skip ci]",
			want:        true,
		},
		{
			description: "matches trailing skip ci",
			message:     "trailing [skip ci]",
			want:        true,
		},
		{
			description: "matches leading skip ci",
			message:     "[skip ci] leading",
			want:        true,
		},
		{
			description: "is case insensitive",
			message:     "case[Skip CI]insensitive",
			want:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			got := resource.ContainsSkipCI(tc.message)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFilterPath(t *testing.T) {
	cases := []struct {
		description string
		pattern     string
		files       []string
		want        []string
	}{
		{
			description: "returns all matching files",
			pattern:     "*.txt",
			files: []string{
				"file1.txt",
				"test/file2.txt",
			},
			want: []string{
				"file1.txt",
			},
		},
		{
			description: "works with wildcard",
			pattern:     "test/*",
			files: []string{
				"file1.txt",
				"test/file2.txt",
			},
			want: []string{
				"test/file2.txt",
			},
		},
		{
			description: "excludes unmatched files",
			pattern:     "*/*.txt",
			files: []string{
				"test/file1.go",
				"test/file2.txt",
			},
			want: []string{
				"test/file2.txt",
			},
		},
		{
			description: "handles prefix matches",
			pattern:     "foo/",
			files: []string{
				"foo/a",
				"foo/a.txt",
				"foo/a/b/c/d.txt",
				"foo",
				"bar",
				"bar/a.txt",
			},
			want: []string{
				"foo/a",
				"foo/a.txt",
				"foo/a/b/c/d.txt",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			got, err := resource.FilterPath(tc.files, tc.pattern)
			if assert.NoError(t, err) {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestFilterIgnorePath(t *testing.T) {
	cases := []struct {
		description string
		pattern     string
		files       []string
		want        []string
	}{
		{
			description: "excludes all matching files",
			pattern:     "*.txt",
			files: []string{
				"file1.txt",
				"test/file2.txt",
			},
			want: []string{
				"test/file2.txt",
			},
		},
		{
			description: "works with wildcard",
			pattern:     "test/*",
			files: []string{
				"file1.txt",
				"test/file2.txt",
			},
			want: []string{
				"file1.txt",
			},
		},
		{
			description: "includes unmatched files",
			pattern:     "*/*.txt",
			files: []string{
				"test/file1.go",
				"test/file2.txt",
			},
			want: []string{
				"test/file1.go",
			},
		},
		{
			description: "handles prefix matches",
			pattern:     "foo/",
			files: []string{
				"foo/a",
				"foo/a.txt",
				"foo/a/b/c/d.txt",
				"foo",
				"bar",
				"bar/a.txt",
			},
			want: []string{
				"foo",
				"bar",
				"bar/a.txt",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			got, err := resource.FilterIgnorePath(tc.files, tc.pattern)
			if assert.NoError(t, err) {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestIsInsidePath(t *testing.T) {
	cases := []struct {
		description string
		parent      string

		expectChildren    []string
		expectNotChildren []string

		want bool
	}{
		{
			description: "basic test",
			parent:      "foo/bar",
			expectChildren: []string{
				"foo/bar",
				"foo/bar/baz",
			},
			expectNotChildren: []string{
				"foo/barbar",
				"foo/baz/bar",
			},
		},
		{
			description: "does not match parent directories against child files",
			parent:      "foo/",
			expectChildren: []string{
				"foo/bar",
			},
			expectNotChildren: []string{
				"foo",
			},
		},
		{
			description: "matches parents without trailing slash",
			parent:      "foo/bar",
			expectChildren: []string{
				"foo/bar",
				"foo/bar/baz",
			},
		},
		{
			description: "handles children that are shorter than the parent",
			parent:      "foo/bar/baz",
			expectNotChildren: []string{
				"foo",
				"foo/bar",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			for _, expectedChild := range tc.expectChildren {
				if !resource.IsInsidePath(tc.parent, expectedChild) {
					t.Errorf("Expected \"%s\" to be inside \"%s\"", expectedChild, tc.parent)
				}
			}

			for _, expectedNotChild := range tc.expectNotChildren {
				if resource.IsInsidePath(tc.parent, expectedNotChild) {
					t.Errorf("Expected \"%s\" to not be inside \"%s\"", expectedNotChild, tc.parent)
				}
			}
		})
	}
}


// i want to test that given i provide json of input args
// that github.ListPullRequests is called with the corect
// arguments. This means using the fake github client
// and the method stub github.ListPullRequestsArgsForCall
// to check that the correct args have been passed
//
// permutations required
//
// MaxPrs
// Page Size
// MaxRetries
// DelayBetweenPages
// SortField
// SortDirection

func TestSetPaginationParameters(t *testing.T) {
	tests := []struct {
		description  string
		inputParameters   resource.Parameters
		expected     resource.Parameters
	}{
		{
			description: "runs with max_prs if specified",
			inputParameters: resource.Parameters{
				MaxPRs:   3,
			},
			expected: resource.Parameters{
				PageSize          : 50,
				MaxPRs            : 3,
				SortField         : "UPDATED_AT",
				SortDirection     : "DESC",
				MaxRetries        : 4,
				DelayBetweenPages : 500,
			},
		},

		{
			description: "runs with page_size if specified",
			inputParameters: resource.Parameters{
				PageSize: 25,
			},
			expected: resource.Parameters{
				PageSize          : 25,
				MaxPRs            : 200,
				SortField         : "UPDATED_AT",
				SortDirection     : "DESC",
				MaxRetries        : 4,
				DelayBetweenPages : 500,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			got := resource.SetPaginationParameters(tc.inputParameters)
			assert.Equal(t, tc.expected, got)
		})
	}

	// t.Run(tc.description, func(t *testing.T) {
	// 	prStates := []githubv4.PullRequestState{githubv4.PullRequestStateOpen}
	// 	github := new(fakes.FakeGithub)
	// 	github.ListPullRequests(prStates, tc.parameters)

	// 	// input := resource.CheckRequest{Source: {}, Version: {"pr": "test"}, Parameters: tc.parameters}
	// 	// output, err := resource.Check(input, github)
	// 	_, output := github.ListPullRequestsArgsForCall(0)

	// 		assert.Equal(t, tc.expected, output)
	// 		// assert.Equal(t, tc.expected, github.ListPullRequestsArgsForCall(prState, tc.parameters))
	// })
	// }

}
