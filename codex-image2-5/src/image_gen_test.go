package main

import "testing"

func TestResolveGPTImageVersions(t *testing.T) {
	cases := map[string]string{
		"":                      "gpt-image-2.5",
		"GPT":                   "gpt-image-2.5",
		"GPT Image":             "gpt-image-2.5",
		"2":                     "gpt-image-2",
		"Image 2":               "gpt-image-2",
		"image2":                "gpt-image-2",
		"Image 二":               "gpt-image-2",
		"2.5":                   "gpt-image-2.5",
		"Image 2.5":             "gpt-image-2.5",
		"二点五":                   "gpt-image-2.5",
		"２．５":                   "gpt-image-2.5",
		"2.6":                   "gpt-image-2.6",
		"Image 2.6":             "gpt-image-2.6",
		"G2.6":                  "gpt-image-2.6",
		"g2.6":                  "gpt-image-2.6",
		"GPT 2.6":               "gpt-image-2.6",
		"以麦 G2.6":               "gpt-image-2.6",
		"gpt-image-2.6":         "gpt-image-2.6",
		"二点六":                   "gpt-image-2.6",
		"两点六":                   "gpt-image-2.6",
		"2.7":                   "gpt-image-2.7",
		"Image 2.7":             "gpt-image-2.7",
		"gpt-image-2.7-flare":   "gpt-image-2.7-flare",
		"2.8":                   "gpt-image-2.8",
		"Image 3":               "gpt-image-3",
		"闪焰":                    "gpt-image-2.5-flare",
		"Image 2.6 闪焰":          "gpt-image-2.6-flare",
		"2.6 flare":             "gpt-image-2.6-flare",
		"日耀":                    "gpt-image-2.5-sunburst",
		"Image 2.6 sunburst":    "gpt-image-2.6-sunburst",
		"最新":                    "chatgpt-image-latest",
		"latest":                "chatgpt-image-latest",
		"Grok":                  "grok-imagine-image-2.0",
		"Grok 高质量":              "grok-imagine-image-quality",
		"custom-image-v7":       "custom-image-v7",
		"gpt-image-2.5-flare":   "gpt-image-2.5-flare",
		"Image 2.5 日耀":          "gpt-image-2.5-sunburst",
	}
	for input, want := range cases {
		got, err := resolveModel(input)
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}
		if got != want {
			t.Fatalf("%q: got %q want %q", input, got, want)
		}
	}
}

func TestRejectVideoOnImagePath(t *testing.T) {
	for _, input := range []string{"高质量", "grok-imagine-video", "Grok 视频"} {
		if _, err := resolveModel(input); err == nil {
			t.Fatalf("expected error for %q", input)
		}
	}
}

func TestResolveVideoModel(t *testing.T) {
	cases := map[string]string{
		"":                 "grok-imagine-video",
		"视频":               "grok-imagine-video",
		"Grok 视频":          "grok-imagine-video",
		"grok-imagine-video": "grok-imagine-video",
		"Grok video 1.5":   "grok-imagine-video-1.5",
		"sora":             "sora-2",
		"sora-2":           "sora-2",
		"kling-v1-video":   "kling-v1-video",
	}
	for input, want := range cases {
		got, err := resolveVideoModel(input)
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}
		if got != want {
			t.Fatalf("%q: got %q want %q", input, got, want)
		}
	}
	if _, err := resolveVideoModel("Image 2.6"); err == nil {
		t.Fatal("expected image model to be rejected on the video path")
	}
}

func TestResolveVideoSizeDefault1K(t *testing.T) {
	pixels, band, _, err := resolveVideoSize("")
	if err != nil {
		t.Fatal(err)
	}
	if pixels != "1280x720" || band != "1K" {
		t.Fatalf("default video size: got %s %s", pixels, band)
	}
	pixels, band, _, err = resolveVideoSize("2K")
	if err != nil {
		t.Fatal(err)
	}
	if pixels != "1920x1080" || band != "2K" {
		t.Fatalf("2K video size: got %s %s", pixels, band)
	}
}
