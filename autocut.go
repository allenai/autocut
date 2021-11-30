package autocut

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v39/github"
)

type Autocut struct {
	Client            *github.Client
	Owner             string
	Repo              string
	AgeThreshold      time.Duration
	ProjectName       string
	ProjectColumnName string
}

type CutResult struct {
	Code     CutCode
	IssueURL string
}

type CutCode string

const (
	None                        CutCode = ""
	IgnoredRecentlyUpdatedIssue         = "found a recently updated issue, so did nothing"
	UpdatedStaleIssue                   = "updated a stale issue"
	ReopenedRecentIssue                 = "re-opened a recently closed issue"
	OpenedNewIssue                      = "opened a new issue"
)

type matchResult string

const (
	foundStaleIssue        matchResult = "found old issue that's open"
	foundRecentIssue                   = "found recent issue that's open"
	foundRecentIssueClosed             = "found recent issue that's closed"
	foundNone                          = "found no issue"
)

const autocutLabel = "autocut"

func (ac *Autocut) Cut(ctx context.Context, title, details string, customLabels []string) (CutResult, error) {
	issues := ac.getIssues(ctx)
	result, matched := ac.firstMatch(issues, title)

	switch result {
	case foundRecentIssue:
		return CutResult{
			Code:     IgnoredRecentlyUpdatedIssue,
			IssueURL: matched.GetHTMLURL(),
		}, nil
	case foundStaleIssue:
		age := time.Now().Sub(matched.GetUpdatedAt())
		update := fmt.Sprintf("It's been %s since the last update (which is more than the threshold of %s), and the problem is still happening.\n\nUpdate:\n\n%s", age.String(), ac.AgeThreshold.String(), details)
		err := ac.comment(ctx, *matched.Number, update)
		if err != nil {
			return CutResult{None, ""}, err
		}
		return CutResult{
			Code:     UpdatedStaleIssue,
			IssueURL: matched.GetHTMLURL(),
		}, nil
	case foundRecentIssueClosed:
		age := time.Now().Sub(matched.GetUpdatedAt())
		err := ac.reopen(ctx, *matched.Number)
		if err != nil {
			return CutResult{None, ""}, err
		}
		update := fmt.Sprintf("Only %s has passed (less than the threshold of %s), and the problem is happening again.\n\nUpdate:\n\n%s", age.String(), ac.AgeThreshold.String(), details)
		err = ac.comment(ctx, *matched.Number, update)
		if err != nil {
			return CutResult{None, ""}, err
		}
		return CutResult{
			Code:     ReopenedRecentIssue,
			IssueURL: matched.GetHTMLURL(),
		}, nil
	case foundNone:
		newIss, err := ac.create(ctx, title, details, customLabels)
		if err != nil {
			return CutResult{None, ""}, err
		}
		return CutResult{
			Code:     OpenedNewIssue,
			IssueURL: newIss.GetHTMLURL(),
		}, nil
	}

	panic("shouldn't get here")
}

func (ac *Autocut) getIssues(ctx context.Context) []*github.Issue {
	opts := &github.IssueListByRepoOptions{
		Labels: []string{autocutLabel},
		State:  "all",
	}

	issues, _, err := ac.Client.Issues.ListByRepo(ctx, ac.Owner, ac.Repo, opts)
	if err != nil {
		panic(err)
	}

	return issues
}

func (ac *Autocut) firstMatch(issues []*github.Issue, title string) (matchResult, *github.Issue) {
	/*
		    Assuming ac.AgeThreshold is 1 day:

			If there's an open autocut issue with this title:
			... and it's been open > 1 day, then add informative comment.

			If there's a closed autocut issue with this title
			... and it's been closed > 1 day, cut a new one.
			... and it's been closed <= 1 day, re-open it and comment on why.
	*/

	for _, i := range issues {
		if title == *i.Title {
			updatedRecently := time.Now().Sub(i.GetUpdatedAt()) < ac.AgeThreshold
			if i.GetState() == "open" {
				if updatedRecently {
					// Recently updated, nothing to do.
					return foundRecentIssue, i
				}
				// Updated a long time ago, but still open. Comment.
				return foundStaleIssue, i
			}
			if i.GetState() == "closed" {
				if updatedRecently {
					// Recently closed, so re-open and comment.
					return foundRecentIssueClosed, i
				} else {
					// Closed a long time ago, so open a new issue.
					return foundNone, nil
				}
			}
		}
	}
	return foundNone, nil
}

func (ac *Autocut) comment(ctx context.Context, issNumber int, message string) error {
	_, _, err := ac.Client.Issues.CreateComment(ctx, ac.Owner, ac.Repo, issNumber, &github.IssueComment{
		Body: &message,
	})

	if err != nil {
		return err
	}

	return nil
}

func (ac *Autocut) reopen(ctx context.Context, issNumber int) error {
	open := "open"
	_, _, err := ac.Client.Issues.Edit(ctx, ac.Owner, ac.Repo, issNumber, &github.IssueRequest{
		State: &open,
	})

	if err != nil {
		return err
	}

	return nil
}

func (ac *Autocut) create(ctx context.Context, title, body string, customLabels []string) (*github.Issue, error) {
	labels := []string{autocutLabel}
	labels = append(labels, customLabels...)

	iss, _, err := ac.Client.Issues.Create(ctx, ac.Owner, ac.Repo, &github.IssueRequest{
		Title:  &title,
		Body:   &body,
		Labels: &labels,
	})
	if err != nil {
		return nil, err
	}

	// Add this new issue to the specified project and column
	if ac.ProjectName != "" && ac.ProjectColumnName != "" {
		err = ac.addToProject(ctx, *iss.ID)
		if err != nil {
			return nil, err
		}
	}

	return iss, nil
}

func (ac *Autocut) addToProject(ctx context.Context, issueID int64) error {
	projectID, err := ac.getProjectID(ctx)
	if err != nil {
		return err
	}

	columnID, err := ac.getColumnID(ctx, projectID)
	if err != nil {
		return err
	}

	_, _, err = ac.Client.Projects.CreateProjectCard(ctx, columnID, &github.ProjectCardOptions{
		Note:        "",
		ContentID:   issueID,
		ContentType: "Issue",
		Archived:    nil,
	})
	if err != nil {
		return fmt.Errorf("couldn't create project card: %w", err)
	}

	return nil
}

func (ac *Autocut) getProjectID(ctx context.Context) (int64, error) {
	page := 0
	for {
		projects, resp, err := ac.Client.Organizations.ListProjects(ctx, ac.Owner, &github.ProjectListOptions{
			State: "all",
			ListOptions: github.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		})
		if err != nil {
			return 0, fmt.Errorf("couldn't fetch project %q: %w", ac.ProjectName, err)
		}

		for _, proj := range projects {
			if *proj.Name == ac.ProjectName {
				return *proj.ID, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return 0, fmt.Errorf("project %q not found", ac.ProjectColumnName)
}

func (ac *Autocut) getColumnID(ctx context.Context, projectID int64) (int64, error) {
	page := 0
	for {
		columns, resp, err := ac.Client.Projects.ListProjectColumns(ctx, projectID, &github.ListOptions{
			Page:    page,
			PerPage: 100,
		})
		if err != nil {
			return 0, fmt.Errorf("couldn't fetch columns of project %d: %w", ac.ProjectName, err)
		}

		for _, column := range columns {
			if *column.Name == ac.ProjectColumnName {
				return *column.ID, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return 0, fmt.Errorf("column %q in project %q not found", ac.ProjectColumnName, ac.ProjectName)
}
