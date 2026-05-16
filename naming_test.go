package main

import "testing"

func TestNormalizeRepoURL(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"ssh github with .git", "git@github.com:ArjenSchwarz/fog.git", "https://github.com/ArjenSchwarz/fog", false},
		{"ssh gitlab", "git@gitlab.com:foo/bar.git", "https://gitlab.com/foo/bar", false},
		{"https with trailing slash", "https://github.com/ArjenSchwarz/fog/", "https://github.com/ArjenSchwarz/fog", false},
		{"http forced to https", "http://gitlab.com/foo/bar.git", "https://gitlab.com/foo/bar", false},
		{"already canonical", "https://github.com/ArjenSchwarz/fog", "https://github.com/ArjenSchwarz/fog", false},
		{"strip query and fragment", "https://github.com/foo/bar?baz=1#anchor", "https://github.com/foo/bar", false},
		{"deep path retained", "https://example.com/a/b/c.git", "https://example.com/a/b/c", false},
		{"no path", "https://github.com", "", true},
		{"no path with slash", "https://github.com/", "", true},
		{"empty", "", "", true},
		{"no host", "https:///foo/bar", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeRepoURL(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestHostLabel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://github.com/foo/bar", "github"},
		{"https://gitlab.example.com/foo/bar", "gitlab"},
		{"https://code.internal/foo/bar", "code"},
	}
	for _, tc := range cases {
		if got := hostLabel(tc.in); got != tc.want {
			t.Errorf("hostLabel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRepoName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://github.com/foo/bar", "bar"},
		{"https://github.com/foo/bar/", "bar"},
		{"https://example.com/a/b/c", "c"},
	}
	for _, tc := range cases {
		if got := repoName(tc.in); got != tc.want {
			t.Errorf("repoName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"feature/foo-bar", "feature-foo-bar"},
		{"Fix/Bug 123", "fix-bug-123"},
		{"arjen/spike--idea/", "arjen-spike-idea"},
		{"--leading-and-trailing--", "leading-and-trailing"},
		{"UPPER_CASE", "upper-case"},
		{"unicode-ñame", "unicode-ame"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := slugify(tc.in); got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPRFilename(t *testing.T) {
	got := prFilename("2026-05-16", "https://github.com/ArjenSchwarz/fog", 42)
	want := "2026-05-16-github-fog-pr42.html"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestBranchFilename(t *testing.T) {
	got := branchFilename("2026-05-16", "https://github.com/ArjenSchwarz/fog", "feature/foo-bar")
	want := "2026-05-16-github-fog-branch-feature-foo-bar.html"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestMonthSubdir(t *testing.T) {
	if got := monthSubdir("2026-05-16T12:00:00Z"); got != "2026-05" {
		t.Errorf("got %q", got)
	}
}

func TestDateSegment(t *testing.T) {
	if got := dateSegment("2026-05-16T12:00:00Z"); got != "2026-05-16" {
		t.Errorf("got %q", got)
	}
}

func TestPRGUID(t *testing.T) {
	got := prGUID("https://github.com/ArjenSchwarz/fog", 42, 1778932800)
	want := "https://github.com/ArjenSchwarz/fog#pr-42-1778932800"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestBranchGUID(t *testing.T) {
	got := branchGUID("https://github.com/ArjenSchwarz/fog", "feature/foo", 1778932800)
	want := "https://github.com/ArjenSchwarz/fog#branch-feature-foo-1778932800"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
