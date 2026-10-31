package main

import "testing"

func TestJoinWithDirs(t *testing.T) {
	tests := []struct {
		dstPath string
		srcPath string
		levels  int
		want    string
	}{
		{"tmp", "/Users/joe/OneDrive/2023/video1.mp4", -1, "tmp/video1.mp4"},
		{"tmp", "/Users/joe/OneDrive/2023/video1.mp4", 0, "tmp/video1.mp4"},
		{"tmp", "/Users/joe/OneDrive/2023/video1.mp4", 1, "tmp/2023/video1.mp4"},
		{"tmp", "/Users/joe/OneDrive/2023/video1.mp4", 5, "tmp/Users/joe/OneDrive/2023/video1.mp4"},
		{"", "/Users/joe/OneDrive/2023/video1.mp4", 0, "video1.mp4"},
		{"", "/Users/joe/OneDrive/2023/video1.mp4", 1, "2023/video1.mp4"},
		{"", "/Users/joe/OneDrive/2023/video1.mp4", 5, "Users/joe/OneDrive/2023/video1.mp4"},
	}

	for i, test := range tests {
		got := joinWithDirs(test.dstPath, test.srcPath, test.levels)
		if got != test.want {
			t.Errorf("test %d: got %q, want %q", i+1, got, test.want)
		}
	}
}
