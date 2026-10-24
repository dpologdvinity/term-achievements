package parse

import (
	"reflect"
	"testing"
)

func normalize(c Cmd) Cmd {
	if c.Args == nil {
		c.Args = []string{}
	}
	if c.SubArgs == nil {
		c.SubArgs = []string{}
	}
	return c
}

func TestParse(t *testing.T) {
	cases := []struct {
		line, prev string
		want       Cmd
	}{
		{"vim notes.md", "", Cmd{Name: "vim", Args: []string{"notes.md"}}},
		{"FOO=1 BAR=2 /usr/bin/make -j8", "", Cmd{Name: "make", Args: []string{"-j8"}}},
		{"sudo -u root vim /etc/hosts", "", Cmd{Name: "vim", Sudo: true, Args: []string{"/etc/hosts"}}},
		{"time nohup env X=1 cargo build", "", Cmd{Name: "cargo", Args: []string{"build"}}},
		{"git -C repo -c a=b --no-pager commit -m x", "", Cmd{Name: "git", Args: []string{"-C", "repo", "-c", "a=b", "--no-pager", "commit", "-m", "x"}, Sub: "commit", SubArgs: []string{"-m", "x"}}},
		{"ls -la | grep foo && echo hi", "", Cmd{Name: "ls", Args: []string{"-la"}}},
		{"\\ls", "", Cmd{Name: "ls", Args: []string{}}},
		{"sudo apt  update", "apt update", Cmd{Name: "apt", Sudo: true, Args: []string{"update"}, SudoRetry: true}},
		{"sudo apt update", "apt upgrade", Cmd{Name: "apt", Sudo: true, Args: []string{"update"}}},
	}
	for _, c := range cases {
		got := Parse(c.line, c.prev)
		if !reflect.DeepEqual(normalize(got), normalize(c.want)) {
			t.Errorf("%q: got %+v want %+v", c.line, got, c.want)
		}
	}
}

func TestParseEdgeCases(t *testing.T) {
	for _, line := range []string{"", "   ", "FOO=1", "sudo", "sudo -u root", "| grep x"} {
		if got := Parse(line, ""); got.Name != "" {
			t.Errorf("%q: got name %q", line, got.Name)
		}
	}
	if got := Parse("sudo", ""); !got.Sudo {
		t.Error("bare sudo still sets Sudo")
	}
}
