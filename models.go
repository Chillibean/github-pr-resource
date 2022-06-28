package resource

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/shurcooL/githubv4"
)

// Source represents the configuration for the resource.
type Source struct {
	Repository              string                      `json:"repository"`
	AccessToken             string                      `json:"access_token"`
	V3Endpoint              string                      `json:"v3_endpoint"`
	V4Endpoint              string                      `json:"v4_endpoint"`
	Paths                   []string                    `json:"paths"`
	IgnorePaths             []string                    `json:"ignore_paths"`
	DisableCISkip           bool                        `json:"disable_ci_skip"`
	DisableGitLFS           bool                        `json:"disable_git_lfs"`
	SkipSSLVerification     bool                        `json:"skip_ssl_verification"`
	DisableForks            bool                        `json:"disable_forks"`
	IgnoreDrafts            bool                        `json:"ignore_drafts"`
	GitCryptKey             string                      `json:"git_crypt_key"`
	BaseBranch              string                      `json:"base_branch"`
	RequiredReviewApprovals int                         `json:"required_review_approvals"`
	Labels                  []string                    `json:"labels"`
	States                  []githubv4.PullRequestState `json:"states"`
	StatusContext           string                      `json:"status_context"`
	Page                    Page                        `json:"page"`
	Verbose                 bool                        `json:"verbose"`
}

// Validate the source configuration.
func (s *Source) Validate() error {
	if s.AccessToken == "" {
		return errors.New("access_token must be set")
	}
	if s.Repository == "" {
		return errors.New("repository must be set")
	}
	if s.V3Endpoint != "" && s.V4Endpoint == "" {
		return errors.New("v4_endpoint must be set together with v3_endpoint")
	}
	if s.V4Endpoint != "" && s.V3Endpoint == "" {
		return errors.New("v3_endpoint must be set together with v4_endpoint")
	}
	for _, state := range s.States {
		switch state {
		case githubv4.PullRequestStateOpen:
		case githubv4.PullRequestStateClosed:
		case githubv4.PullRequestStateMerged:
		default:
			return errors.New(fmt.Sprintf("states value \"%s\" must be one of: OPEN, MERGED, CLOSED", state))
		}
	}
	if s.Verbose {
		os.Setenv("verbose", "true")
	}
	if err := s.Page.Validate(); err != nil {
		return err
	}

	return nil
}

// Metadata output from get/put steps.
type Metadata []*MetadataField

// Add a MetadataField to the Metadata.
func (m *Metadata) Add(name, value string) {
	*m = append(*m, &MetadataField{Name: name, Value: value})
}

// MetadataField ...
type MetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Version communicated with Concourse.
type Version struct {
	PR                  string                    `json:"pr"`
	Commit              string                    `json:"commit"`
	CommittedDate       time.Time                 `json:"committed,omitempty"`
	ApprovedReviewCount string                    `json:"approved_review_count"`
	State               githubv4.PullRequestState `json:"state"`
}

// NewVersion constructs a new Version.
func NewVersion(p *PullRequest) Version {
	return Version{
		PR:                  strconv.Itoa(p.Number),
		Commit:              p.Tip.OID,
		CommittedDate:       p.UpdatedDate().Time,
		ApprovedReviewCount: strconv.Itoa(p.ApprovedReviewCount),
		State:               p.State,
	}
}

// PullRequest represents a pull request and includes the tip (commit).
type PullRequest struct {
	PullRequestObject
	Tip                 CommitObject
	ApprovedReviewCount int
	Labels              []LabelObject
	HasStatus           bool
}

// PullRequestObject represents the GraphQL commit node.
// https://developer.github.com/v4/object/pullrequest/
type PullRequestObject struct {
	ID          string
	Number      int
	Title       string
	URL         string
	BaseRefName string
	HeadRefName string
	Repository  struct {
		URL string
	}
	IsCrossRepository bool
	IsDraft           bool
	State             githubv4.PullRequestState
	ClosedAt          githubv4.DateTime
	MergedAt          githubv4.DateTime
}

// UpdatedDate returns the last time a PR was updated, either by commit
// or being closed/merged.
func (p *PullRequest) UpdatedDate() githubv4.DateTime {
	date := p.Tip.CommittedDate
	switch p.State {
	case githubv4.PullRequestStateClosed:
		date = p.ClosedAt
	case githubv4.PullRequestStateMerged:
		date = p.MergedAt
	}
	return date
}

// CommitObject represents the GraphQL commit node.
// https://developer.github.com/v4/object/commit/
type CommitObject struct {
	ID            string
	OID           string
	CommittedDate githubv4.DateTime
	Message       string
	Author        struct {
		User struct {
			Login string
		}
		Email string
	}
}

// StatusObject represents the GraphQL FilesChanged node.
// https://developer.github.com/v4/object/status/
type StatusObject struct {
	Context struct {
		Context *githubv4.String
	} `graphql:"context(name:$statusContextName)"`
}

// ChangedFileObject represents the GraphQL FilesChanged node.
// https://developer.github.com/v4/object/pullrequestchangedfile/
type ChangedFileObject struct {
	Path string
}

// LabelObject represents the GraphQL label node.
// https://developer.github.com/v4/object/label
type LabelObject struct {
	Name string
}

// Page represents settings for request parameters
type Page struct {
	PageSize          int                      `json:"page_size"`
	MaxPRs            int                      `json:"max_prs"`
	SortField         githubv4.IssueOrderField `json:"sort_field"`
	SortDirection     githubv4.OrderDirection  `json:"sort_direction"`
	MaxRetries        int                      `json:"max_retries"`
	DelayBetweenPages int                      `json:"delay_between_pages"`
}

// Validate Page configuration
func (p *Page) Validate() error {
	if p.MaxPRs <= 0 {
		p.MaxPRs = 100
	} else if p.MaxPRs > 2000 {
		p.MaxPRs = 2000
		fmt.Println("Max max_prs value exceeded, using max value 2000")
	}

	if p.PageSize <= 0 {
		p.PageSize = 50
	} else if p.PageSize > p.MaxPRs {
		p.PageSize = p.MaxPRs
	}
	if p.PageSize > 100 {
		p.PageSize = 100
		fmt.Println("Max page_size exceeded, using max value 100")
	}

	if p.MaxRetries <= 0 {
		p.MaxRetries = 4
	}

	if p.DelayBetweenPages <= 0 {
		p.DelayBetweenPages = 500
	}

	switch p.SortField {
	case "":
		p.SortField = githubv4.IssueOrderFieldUpdatedAt
	case githubv4.IssueOrderFieldCreatedAt:
	case githubv4.IssueOrderFieldUpdatedAt:
	case githubv4.IssueOrderFieldComments:
	default:
		return errors.New(fmt.Sprintf("sort_field '%s' not valid, please choose one of 'UPDATED_AT', 'CREATED_AT' or 'COMMENTS'", p.SortField))
	}

	switch p.SortDirection {
	case "":
		p.SortDirection = githubv4.OrderDirectionDesc
	case githubv4.OrderDirectionAsc:
	case githubv4.OrderDirectionDesc:
	default:
		return errors.New(fmt.Sprintf("sort_dir '%s' not valid, please choose one of 'ASC' or 'DESC'", p.SortDirection))
	}

	return nil
}
