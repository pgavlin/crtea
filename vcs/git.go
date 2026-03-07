package vcs

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pgavlin/crtea/model"
)

// GitBackend implements Backend using the git CLI.
type GitBackend struct {
	info VcsInfo
}

// NewGitBackend creates a new Git backend for the given directory.
func NewGitBackend(dir string) (*GitBackend, error) {
	rootPath, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	headCommit, _ := gitOutput(dir, "rev-parse", "--short", "HEAD")
	branchName, _ := gitOutput(dir, "symbolic-ref", "--short", "HEAD")

	return &GitBackend{
		info: VcsInfo{
			RootPath:   strings.TrimSpace(rootPath),
			HeadCommit: strings.TrimSpace(headCommit),
			BranchName: strings.TrimSpace(branchName),
			VcsType:    "git",
		},
	}, nil
}

func (g *GitBackend) Info() VcsInfo {
	return g.info
}

func (g *GitBackend) GetWorkingTreeDiff() ([]model.DiffFile, error) {
	// Get both staged and unstaged changes
	out, err := gitOutput(g.info.RootPath, "diff", "HEAD", "--unified=3", "--no-color", "--src-prefix=a/", "--dst-prefix=b/")
	if err != nil {
		// If HEAD doesn't exist (new repo), diff against empty tree
		out, err = gitOutput(g.info.RootPath, "diff", "--cached", "--unified=3", "--no-color", "--src-prefix=a/", "--dst-prefix=b/")
		if err != nil {
			return nil, err
		}
	}
	return parseDiff(out), nil
}

func (g *GitBackend) GetRevisionDiff(revSpec string) ([]model.DiffFile, error) {
	args := []string{"diff", revSpec, "--unified=3", "--no-color", "--src-prefix=a/", "--dst-prefix=b/"}
	out, err := gitOutput(g.info.RootPath, args...)
	if err != nil {
		return nil, err
	}
	return parseDiff(out), nil
}

func (g *GitBackend) GetCommitRangeDiff(ids []string) ([]model.DiffFile, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var args []string
	if len(ids) == 1 {
		args = []string{"diff", ids[0] + "^", ids[0], "--unified=3", "--no-color", "--src-prefix=a/", "--dst-prefix=b/"}
	} else {
		args = []string{"diff", ids[0] + "^", ids[len(ids)-1], "--unified=3", "--no-color", "--src-prefix=a/", "--dst-prefix=b/"}
	}
	out, err := gitOutput(g.info.RootPath, args...)
	if err != nil {
		return nil, err
	}
	return parseDiff(out), nil
}

func (g *GitBackend) FetchContextLines(filePath string, status model.FileStatus, startLine, endLine int) ([]model.DiffLine, error) {
	var content string
	var err error

	if status == model.FileDeleted {
		content, err = gitOutput(g.info.RootPath, "show", "HEAD:"+filePath)
	} else {
		fullPath := filepath.Join(g.info.RootPath, filePath)
		data, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			return nil, readErr
		}
		content = string(data)
	}
	if err != nil {
		return nil, err
	}

	lines := strings.Split(content, "\n")
	var result []model.DiffLine
	for i := startLine - 1; i < endLine && i < len(lines); i++ {
		result = append(result, model.DiffLine{
			Origin:    model.OriginContext,
			Content:   lines[i],
			OldLineNo: i + 1,
			NewLineNo: i + 1,
		})
	}
	return result, nil
}

func (g *GitBackend) GetRecentCommits(offset, limit int) ([]CommitInfo, error) {
	format := "%H%n%h%n%s%n%an%n%aI%n%D"
	out, err := gitOutput(g.info.RootPath, "log",
		fmt.Sprintf("--skip=%d", offset),
		fmt.Sprintf("--max-count=%d", limit),
		fmt.Sprintf("--format=%s", format),
	)
	if err != nil {
		return nil, err
	}

	var commits []CommitInfo
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := 0; i+5 < len(lines); i += 6 {
		t, _ := time.Parse(time.RFC3339, lines[i+4])
		branch := ""
		refs := lines[i+5]
		if strings.Contains(refs, "HEAD -> ") {
			parts := strings.SplitAfter(refs, "HEAD -> ")
			if len(parts) > 1 {
				branch = strings.Split(parts[1], ",")[0]
				branch = strings.TrimSpace(branch)
			}
		}
		commits = append(commits, CommitInfo{
			ID:         lines[i],
			ShortID:    lines[i+1],
			Summary:    lines[i+2],
			Author:     lines[i+3],
			Time:       t,
			BranchName: branch,
		})
	}
	return commits, nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// parseDiff parses unified diff output into DiffFile structs.
func parseDiff(input string) []model.DiffFile {
	var files []model.DiffFile
	var currentFile *model.DiffFile
	var currentHunk *model.DiffHunk

	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "diff --git ") {
			if currentFile != nil {
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
					currentHunk = nil
				}
				files = append(files, *currentFile)
			}
			currentFile = &model.DiffFile{Status: model.FileModified}
			parts := strings.SplitN(line, " b/", 2)
			if len(parts) == 2 {
				currentFile.NewPath = parts[1]
			}
			aParts := strings.SplitN(line, " a/", 2)
			if len(aParts) == 2 {
				oldPath := strings.SplitN(aParts[1], " b/", 2)
				if len(oldPath) > 0 {
					currentFile.OldPath = oldPath[0]
				}
			}
			continue
		}

		if currentFile == nil {
			continue
		}

		if strings.HasPrefix(line, "new file mode") {
			currentFile.Status = model.FileAdded
			continue
		}
		if strings.HasPrefix(line, "deleted file mode") {
			currentFile.Status = model.FileDeleted
			continue
		}
		if strings.HasPrefix(line, "rename from ") {
			currentFile.OldPath = strings.TrimPrefix(line, "rename from ")
			currentFile.Status = model.FileRenamed
			continue
		}
		if strings.HasPrefix(line, "rename to ") {
			currentFile.NewPath = strings.TrimPrefix(line, "rename to ")
			continue
		}
		if strings.HasPrefix(line, "Binary files") {
			currentFile.IsBinary = true
			continue
		}
		if strings.HasPrefix(line, "--- ") {
			if strings.HasPrefix(line, "--- a/") {
				currentFile.OldPath = strings.TrimPrefix(line, "--- a/")
			}
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			if strings.HasPrefix(line, "+++ b/") {
				currentFile.NewPath = strings.TrimPrefix(line, "+++ b/")
			}
			continue
		}

		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			}
			currentHunk = parseHunkHeader(line)
			continue
		}

		if currentHunk == nil {
			continue
		}

		dl := model.DiffLine{Content: safeTrimPrefix(line)}
		switch {
		case strings.HasPrefix(line, "+"):
			dl.Origin = model.OriginAddition
			dl.NewLineNo = currentHunk.NewStart + countLines(currentHunk.Lines, model.OriginAddition, model.OriginContext)
		case strings.HasPrefix(line, "-"):
			dl.Origin = model.OriginDeletion
			dl.OldLineNo = currentHunk.OldStart + countLines(currentHunk.Lines, model.OriginDeletion, model.OriginContext)
		default:
			dl.Origin = model.OriginContext
			dl.OldLineNo = currentHunk.OldStart + countLines(currentHunk.Lines, model.OriginDeletion, model.OriginContext)
			dl.NewLineNo = currentHunk.NewStart + countLines(currentHunk.Lines, model.OriginAddition, model.OriginContext)
		}
		currentHunk.Lines = append(currentHunk.Lines, dl)
	}

	if currentFile != nil {
		if currentHunk != nil {
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		}
		files = append(files, *currentFile)
	}
	return files
}

func safeTrimPrefix(line string) string {
	if len(line) > 0 {
		return line[1:]
	}
	return ""
}

func countLines(lines []model.DiffLine, origins ...model.LineOrigin) int {
	count := 0
	for _, l := range lines {
		for _, o := range origins {
			if l.Origin == o {
				count++
				break
			}
		}
	}
	return count
}

func parseHunkHeader(line string) *model.DiffHunk {
	hunk := &model.DiffHunk{Header: line}
	// Parse @@ -oldStart,oldCount +newStart,newCount @@
	parts := strings.SplitN(line, "@@", 3)
	if len(parts) < 2 {
		return hunk
	}
	ranges := strings.TrimSpace(parts[1])
	rangeParts := strings.Fields(ranges)
	for _, rp := range rangeParts {
		if strings.HasPrefix(rp, "-") {
			nums := strings.SplitN(strings.TrimPrefix(rp, "-"), ",", 2)
			hunk.OldStart, _ = strconv.Atoi(nums[0])
			if len(nums) > 1 {
				hunk.OldCount, _ = strconv.Atoi(nums[1])
			} else {
				hunk.OldCount = 1
			}
		} else if strings.HasPrefix(rp, "+") {
			nums := strings.SplitN(strings.TrimPrefix(rp, "+"), ",", 2)
			hunk.NewStart, _ = strconv.Atoi(nums[0])
			if len(nums) > 1 {
				hunk.NewCount, _ = strconv.Atoi(nums[1])
			} else {
				hunk.NewCount = 1
			}
		}
	}
	return hunk
}
