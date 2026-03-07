package github

import "testing"

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		url         string
		wantOwner   string
		wantRepo    string
		wantErr     bool
	}{
		{"git@github.com:octocat/hello-world.git", "octocat", "hello-world", false},
		{"git@github.com:octocat/hello-world", "octocat", "hello-world", false},
		{"https://github.com/octocat/hello-world.git", "octocat", "hello-world", false},
		{"https://github.com/octocat/hello-world", "octocat", "hello-world", false},
		{"ssh://git@github.com/octocat/hello-world.git", "octocat", "hello-world", false},
		{"github.com/octocat/hello-world", "octocat", "hello-world", false},
		{"https://gitlab.com/octocat/hello-world.git", "", "", true},
		{"git@gitlab.com:octocat/hello-world.git", "", "", true},
		{"not-a-url", "", "", true},
	}

	for _, tt := range tests {
		owner, repo, err := ParseRemoteURL(tt.url)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseRemoteURL(%q): expected error, got %s/%s", tt.url, owner, repo)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRemoteURL(%q): unexpected error: %v", tt.url, err)
			continue
		}
		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("ParseRemoteURL(%q) = %s/%s, want %s/%s", tt.url, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}
