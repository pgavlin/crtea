package model

// FileStatus represents the status of a file in a diff.
type FileStatus int

const (
	FileAdded FileStatus = iota
	FileModified
	FileDeleted
	FileRenamed
	FileCopied
)

func (s FileStatus) String() string {
	switch s {
	case FileAdded:
		return "A"
	case FileModified:
		return "M"
	case FileDeleted:
		return "D"
	case FileRenamed:
		return "R"
	case FileCopied:
		return "C"
	default:
		return "?"
	}
}

// LineOrigin indicates whether a diff line is context, addition, or deletion.
type LineOrigin int

const (
	OriginContext LineOrigin = iota
	OriginAddition
	OriginDeletion
)

// DiffLine represents a single line in a diff hunk.
type DiffLine struct {
	Origin   LineOrigin
	Content  string
	OldLineNo int // 0 means not applicable
	NewLineNo int // 0 means not applicable
}

// DiffHunk represents a hunk in a diff.
type DiffHunk struct {
	Header   string
	Lines    []DiffLine
	OldStart int
	OldCount int
	NewStart int
	NewCount int
}

// DiffFile represents a single file's diff.
type DiffFile struct {
	OldPath  string
	NewPath  string
	Status   FileStatus
	Hunks    []DiffHunk
	IsBinary bool
}

// DisplayPath returns the path to display for this file.
func (f *DiffFile) DisplayPath() string {
	if f.NewPath != "" {
		return f.NewPath
	}
	return f.OldPath
}
