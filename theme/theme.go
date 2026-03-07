package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme defines all colors used in the UI.
type Theme struct {
	// Base
	PanelBg     color.Color
	BgHighlight color.Color
	FgPrimary   color.Color
	FgSecondary color.Color
	FgDim       color.Color

	// Diff
	DiffAdd       color.Color
	DiffAddBg     color.Color
	DiffDel       color.Color
	DiffDelBg     color.Color
	DiffContext   color.Color
	DiffHunkHdr   color.Color
	ExpandedCtxFg color.Color

	// File status
	FileAdded    color.Color
	FileModified color.Color
	FileDeleted  color.Color
	FileRenamed  color.Color

	// Review
	Reviewed color.Color
	Pending  color.Color

	// Comments
	CommentNote       color.Color
	CommentSuggestion color.Color
	CommentIssue      color.Color
	CommentPraise     color.Color

	// UI
	BorderFocused   color.Color
	BorderUnfocused color.Color
	StatusBarBg     color.Color
	StatusBarFg     color.Color
	CursorColor     color.Color
	BranchName      color.Color

	// Messages
	MessageInfoFg  color.Color
	MessageInfoBg  color.Color
	MessageWarnFg  color.Color
	MessageWarnBg  color.Color
	MessageErrorFg color.Color
	MessageErrorBg color.Color

	// Mode indicator
	ModeFg color.Color
	ModeBg color.Color
}

func c(hex string) color.Color {
	return lipgloss.Color(hex)
}

// Dark returns the dark theme.
func Dark() Theme {
	return Theme{
		PanelBg:     c("#1e1e2e"),
		BgHighlight: c("#313244"),
		FgPrimary:   c("#cdd6f4"),
		FgSecondary: c("#a6adc8"),
		FgDim:       c("#585b70"),

		DiffAdd:       c("#a6e3a1"),
		DiffAddBg:     c("#1a3a1a"),
		DiffDel:       c("#f38ba8"),
		DiffDelBg:     c("#3a1a1a"),
		DiffContext:   c("#bac2de"),
		DiffHunkHdr:   c("#89b4fa"),
		ExpandedCtxFg: c("#6c7086"),

		FileAdded:    c("#a6e3a1"),
		FileModified: c("#f9e2af"),
		FileDeleted:  c("#f38ba8"),
		FileRenamed:  c("#89b4fa"),

		Reviewed: c("#a6e3a1"),
		Pending:  c("#f9e2af"),

		CommentNote:       c("#89b4fa"),
		CommentSuggestion: c("#a6e3a1"),
		CommentIssue:      c("#f38ba8"),
		CommentPraise:     c("#f9e2af"),

		BorderFocused:   c("#89b4fa"),
		BorderUnfocused: c("#45475a"),
		StatusBarBg:     c("#181825"),
		StatusBarFg:     c("#cdd6f4"),
		CursorColor:     c("#89b4fa"),
		BranchName:      c("#cba6f7"),

		MessageInfoFg:  c("#1e1e2e"),
		MessageInfoBg:  c("#89b4fa"),
		MessageWarnFg:  c("#1e1e2e"),
		MessageWarnBg:  c("#f9e2af"),
		MessageErrorFg: c("#1e1e2e"),
		MessageErrorBg: c("#f38ba8"),

		ModeFg: c("#1e1e2e"),
		ModeBg: c("#89b4fa"),
	}
}

// Light returns the light theme.
func Light() Theme {
	return Theme{
		PanelBg:     c("#eff1f5"),
		BgHighlight: c("#ccd0da"),
		FgPrimary:   c("#4c4f69"),
		FgSecondary: c("#6c6f85"),
		FgDim:       c("#9ca0b0"),

		DiffAdd:       c("#40a02b"),
		DiffAddBg:     c("#d5f0cd"),
		DiffDel:       c("#d20f39"),
		DiffDelBg:     c("#f5d0d8"),
		DiffContext:   c("#4c4f69"),
		DiffHunkHdr:   c("#1e66f5"),
		ExpandedCtxFg: c("#9ca0b0"),

		FileAdded:    c("#40a02b"),
		FileModified: c("#df8e1d"),
		FileDeleted:  c("#d20f39"),
		FileRenamed:  c("#1e66f5"),

		Reviewed: c("#40a02b"),
		Pending:  c("#df8e1d"),

		CommentNote:       c("#1e66f5"),
		CommentSuggestion: c("#40a02b"),
		CommentIssue:      c("#d20f39"),
		CommentPraise:     c("#df8e1d"),

		BorderFocused:   c("#1e66f5"),
		BorderUnfocused: c("#bcc0cc"),
		StatusBarBg:     c("#dce0e8"),
		StatusBarFg:     c("#4c4f69"),
		CursorColor:     c("#1e66f5"),
		BranchName:      c("#8839ef"),

		MessageInfoFg:  c("#eff1f5"),
		MessageInfoBg:  c("#1e66f5"),
		MessageWarnFg:  c("#eff1f5"),
		MessageWarnBg:  c("#df8e1d"),
		MessageErrorFg: c("#eff1f5"),
		MessageErrorBg: c("#d20f39"),

		ModeFg: c("#eff1f5"),
		ModeBg: c("#1e66f5"),
	}
}
