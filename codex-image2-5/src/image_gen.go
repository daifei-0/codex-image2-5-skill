package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultAPIURL      = "https://apinebula.com"
	defaultModel       = "gpt-image-2.5"
	defaultGrokModel   = "grok-imagine-image-2.0"
	defaultVideoModel  = "grok-imagine-video"
	defaultSize        = "1K"
	defaultQuality     = "auto"
	defaultOutDir      = "output/imagegen"
	defaultVideoOutDir = "output/videogen"
	defaultVideoSecs   = 8
)

// sizePresets 把用户说的 1K/2K/4K 换成 Images API 真正认的像素。
// 4K 用 3840x2160：官方长边上限 3840，正方形 3840x3840 会超过像素总量上限。
var sizePresets = map[string]string{
	"1K": "1024x1024",
	"2K": "2048x2048",
	"4K": "3840x2160",
}

var retryable = map[int]bool{429: true, 500: true, 502: true, 503: true, 504: true, 524: true}

// Grok 别名仍用表。GPT Image 版本号（2.5、2.6、2.7…）由 resolveGPTImage 按规则生成，不必每次改表。
var modelAliases = []struct {
	alias string
	id    string
}{
	{"grok-imagine-image-quality", "grok-imagine-image-quality"},
	{"grok imagine image quality", "grok-imagine-image-quality"},
	{"grok imagine quality", "grok-imagine-image-quality"},
	{"grok quality", "grok-imagine-image-quality"},
	{"imagine quality", "grok-imagine-image-quality"},
	{"grok 高质量", "grok-imagine-image-quality"},
	{"grok 高质量版", "grok-imagine-image-quality"},
	{"grok 质量版", "grok-imagine-image-quality"},
	{"grok-imagine-image-2.0", "grok-imagine-image-2.0"},
	{"grok imagine image 2.0", "grok-imagine-image-2.0"},
	{"grok imagine 2.0", "grok-imagine-image-2.0"},
	{"grok imagine image 2", "grok-imagine-image-2.0"},
	{"grok 2.0", "grok-imagine-image-2.0"},
	{"grok 2", "grok-imagine-image-2.0"},
	{"imagine 2.0", "grok-imagine-image-2.0"},
	{"grok-imagine", "grok-imagine"},
	{"grok imagine", "grok-imagine"},
	{"grok", defaultGrokModel},
	{"chatgpt-image-latest", "chatgpt-image-latest"},
	{"chatgpt image latest", "chatgpt-image-latest"},
	{"chatgpt latest", "chatgpt-image-latest"},
	{"image latest", "chatgpt-image-latest"},
	{"latest", "chatgpt-image-latest"},
	{"最新", "chatgpt-image-latest"},
	{"最新模型", "chatgpt-image-latest"},
}

type commonArgs struct {
	model       string
	size        string
	quality     string
	n           int
	outDir      string
	force       bool
	dryRun      bool
	maxAttempts int
	timeout     time.Duration
}

type apiResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
}

type batchJob struct {
	Prompt  string `json:"prompt"`
	Model   string `json:"model,omitempty"`
	Size    string `json:"size,omitempty"`
	Quality string `json:"quality,omitempty"`
	N       int    `json:"n,omitempty"`
	Out     string `json:"out,omitempty"`
}

func endpoint(base, operation string) string {
	return apiRoot(base) + "/images/" + operation
}

// resolveSize 接受 1K/2K/4K、auto，或 WIDTHxHEIGHT。发给上游的永远是后两种。
func resolveSize(size string) (string, error) {
	size = normalizeSize(size)
	if size == "" {
		size = defaultSize
	}
	if preset, ok := sizePresets[strings.ToUpper(size)]; ok {
		return preset, nil
	}
	if strings.EqualFold(size, "auto") {
		return "auto", nil
	}
	parts := strings.Split(strings.ToLower(size), "x")
	if len(parts) != 2 {
		return "", errors.New("size must be 1K, 2K, 4K, auto, or WIDTHxHEIGHT")
	}
	w, e1 := strconv.Atoi(parts[0])
	h, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || w < 1 || h < 1 {
		return "", errors.New("size must be 1K, 2K, 4K, auto, or WIDTHxHEIGHT")
	}
	return fmt.Sprintf("%dx%d", w, h), nil
}

func normalizeWidth(value string) string {
	return strings.Map(func(character rune) rune {
		if character >= '！' && character <= '～' {
			return character - 0xFEE0
		}
		return character
	}, value)
}

func normalizeKey(value string) string {
	value = strings.ToLower(normalizeWidth(value))
	value = strings.NewReplacer("-", "", "_", "").Replace(value)
	return strings.Join(strings.Fields(value), "")
}

func normalizeSize(size string) string {
	size = strings.ToUpper(strings.Join(strings.Fields(normalizeWidth(size)), ""))
	size = strings.ReplaceAll(size, "×", "X")
	switch size {
	case "一K":
		return "1K"
	case "二K", "两K":
		return "2K"
	case "四K":
		return "4K"
	}
	return size
}

func isVideoModel(model string) bool {
	key := normalizeKey(model)
	return strings.Contains(key, "video") || strings.Contains(key, "视频")
}

func isGrokImage(model string) bool {
	k := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(k, "grok-imagine") && !isVideoModel(k)
}

func currentDefaultModel() string {
	if value := strings.TrimSpace(os.Getenv("CODEX_IMAGE_DEFAULT_MODEL")); value != "" {
		return value
	}
	return defaultModel
}

func currentDefaultVideoModel() string {
	if value := strings.TrimSpace(os.Getenv("CODEX_VIDEO_DEFAULT_MODEL")); value != "" {
		return value
	}
	return defaultVideoModel
}

func defaultGPTBase() string {
	id := strings.ToLower(strings.TrimSpace(currentDefaultModel()))
	id = strings.TrimSuffix(id, "-flare")
	id = strings.TrimSuffix(id, "-sunburst")
	if strings.HasPrefix(id, "gpt-image-") {
		return id
	}
	return defaultModel
}

func rewriteSpokenDigits(value string) string {
	var built strings.Builder
	for _, character := range value {
		switch character {
		case '零', '〇', '○':
			built.WriteByte('0')
		case '一', '壹':
			built.WriteByte('1')
		case '二', '两', '兩', '贰':
			built.WriteByte('2')
		case '三', '叁':
			built.WriteByte('3')
		case '四', '肆':
			built.WriteByte('4')
		case '五', '伍':
			built.WriteByte('5')
		case '六', '陆', '陸':
			built.WriteByte('6')
		case '七', '柒':
			built.WriteByte('7')
		case '八', '捌':
			built.WriteByte('8')
		case '九', '玖':
			built.WriteByte('9')
		case '点', '點', '．':
			built.WriteByte('.')
		default:
			built.WriteRune(character)
		}
	}
	return built.String()
}

func splitGPTVariant(key string) (rest, variant string) {
	type suffix struct {
		token string
		id    string
	}
	for _, item := range []suffix{
		{normalizeKey("闪焰版"), "-flare"},
		{normalizeKey("日耀版"), "-sunburst"},
		{normalizeKey("flare"), "-flare"},
		{normalizeKey("sunburst"), "-sunburst"},
		{normalizeKey("闪焰"), "-flare"},
		{normalizeKey("日耀"), "-sunburst"},
	} {
		if item.token != "" && strings.HasSuffix(key, item.token) {
			return strings.TrimSuffix(key, item.token), item.id
		}
	}
	return key, ""
}

func stripGPTPrefix(key string) string {
	for _, prefix := range []string{"以麦", "gptimage", "gpt", "image", "img"} {
		if strings.HasPrefix(key, prefix) {
			key = strings.TrimPrefix(key, prefix)
			break
		}
	}
	if strings.HasPrefix(key, "g") && looksLikeVersion(key[1:]) {
		return key[1:]
	}
	return key
}

func gptImageID(version, variant string) string {
	if version == "" {
		return defaultGPTBase() + variant
	}
	major, minor, ok := strings.Cut(version, ".")
	if !ok || minor == "0" {
		return "gpt-image-" + major + variant
	}
	return "gpt-image-" + version + variant
}

func looksLikeVersion(value string) bool {
	if value == "" {
		return false
	}
	major, minor, ok := strings.Cut(value, ".")
	if !ok {
		_, err := strconv.Atoi(value)
		return err == nil
	}
	if major == "" || minor == "" {
		return false
	}
	_, e1 := strconv.Atoi(major)
	_, e2 := strconv.Atoi(minor)
	return e1 == nil && e2 == nil
}

func resolveGPTImage(model string) (string, bool) {
	key := normalizeKey(rewriteSpokenDigits(model))
	rest, variant := splitGPTVariant(key)
	rest = stripGPTPrefix(rest)
	if rest == "" {
		if variant != "" || key == "gpt" || key == "gptimage" {
			return gptImageID("", variant), true
		}
		return "", false
	}
	if !looksLikeVersion(rest) {
		return "", false
	}
	return gptImageID(rest, variant), true
}

func apiRoot(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		base = defaultAPIURL
	}
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	return base
}

// resolveModel 把对话里的叫法收成网关生图模型名。视频模型请用 video 命令；认不出的原样传。
func resolveModel(model string) (string, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return currentDefaultModel(), nil
	}
	if isVideoModel(model) {
		return "", errors.New("this is a video model; use the video command")
	}
	key := normalizeKey(rewriteSpokenDigits(model))
	if key == "高质量" || key == "高清" || key == "highquality" {
		return "", errors.New("quality alone does not select a model; keep the current model and use --quality high, or explicitly choose Grok 高质量")
	}
	for _, item := range modelAliases {
		if key == normalizeKey(item.alias) {
			return item.id, nil
		}
	}
	if id, ok := resolveGPTImage(model); ok {
		return id, nil
	}
	return model, nil
}

func resolveVideoModel(model string) (string, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return currentDefaultVideoModel(), nil
	}
	key := normalizeKey(rewriteSpokenDigits(model))
	if key == "高质量" || key == "高清" || key == "highquality" {
		return "", errors.New("quality alone does not select a video model")
	}
	for _, item := range []struct {
		alias string
		id    string
	}{
		{"grok-imagine-video-1.5-preview", "grok-imagine-video-1.5-preview"},
		{"grok-imagine-video-1.5", "grok-imagine-video-1.5"},
		{"grok imagine video 1.5", "grok-imagine-video-1.5"},
		{"grok video 1.5", "grok-imagine-video-1.5"},
		{"grok 视频 1.5", "grok-imagine-video-1.5"},
		{"grok-imagine-video", "grok-imagine-video"},
		{"grok imagine video", "grok-imagine-video"},
		{"grok video", "grok-imagine-video"},
		{"grok 视频", "grok-imagine-video"},
		{"sora-2", "sora-2"},
		{"sora2", "sora-2"},
		{"sora 2", "sora-2"},
		{"sora", "sora-2"},
		{"video", currentDefaultVideoModel()},
		{"视频", currentDefaultVideoModel()},
		{"生成视频", currentDefaultVideoModel()},
	} {
		if key == normalizeKey(item.alias) {
			return item.id, nil
		}
	}
	if isVideoModel(model) {
		return model, nil
	}
	if id, ok := resolveGPTImage(model); ok {
		return "", fmt.Errorf("%s is an image model; use generate, not video", id)
	}
	return model, nil
}

func resolveVideoSize(size string) (pixels, band, note string, err error) {
	size = normalizeSize(size)
	if size == "" {
		size = defaultSize
	}
	switch strings.ToUpper(size) {
	case "1K", "AUTO", "720P", "720":
		return "1280x720", "1K", "", nil
	case "2K", "1080P", "1080":
		return "1920x1080", "2K", "", nil
	case "4K", "2160P", "2160":
		return "1920x1080", "4K", "most video models top out at 1080p; requested the highest common slot", nil
	case "480P", "480":
		return "848x480", "480p", "", nil
	}
	if strings.Contains(strings.ToLower(size), "x") {
		parts := strings.Split(strings.ToLower(size), "x")
		if len(parts) == 2 {
			w, e1 := strconv.Atoi(parts[0])
			h, e2 := strconv.Atoi(parts[1])
			if e1 == nil && e2 == nil && w > 0 && h > 0 {
				return fmt.Sprintf("%dx%d", w, h), requestedBand(size), "", nil
			}
		}
	}
	return "", "", "", errors.New("video size must be 1K, 2K, 4K, 480p/720p/1080p, or WIDTHxHEIGHT")
}

func grokResolution(size string) (res, pixel, note string) {
	size = normalizeSize(size)
	if size == "" {
		size = defaultSize
	}
	switch strings.ToUpper(size) {
	case "1K", "AUTO":
		return "1k", "1024x1024", ""
	case "2K":
		return "2k", "2048x2048", ""
	case "4K":
		// Do not send resolution=4k: 6rr ignores it and returns 1024x1024.
		// size=3840x2160 is the highest slot (typically 2816x1584).
		return "2k", "3840x2160", "Grok Imagine has no 4K; requested the highest available slot"
	}
	resolved, err := resolveSize(size)
	if err != nil {
		return "1k", "1024x1024", ""
	}
	parts := strings.Split(strings.ToLower(resolved), "x")
	if len(parts) == 2 {
		w, e1 := strconv.Atoi(parts[0])
		h, e2 := strconv.Atoi(parts[1])
		if e1 == nil && e2 == nil && (w >= 3000 || h >= 3000) {
			return "2k", resolved, ""
		}
		if e1 == nil && e2 == nil && (w >= 1536 || h >= 1536) {
			return "2k", resolved, ""
		}
	}
	return "1k", "1024x1024", ""
}

type imageSpec struct {
	model   string
	payload map[string]any
	fields  map[string]string
	extra   map[string]any
}

func prepareImageRequest(prompt string, args commonArgs) (imageSpec, error) {
	model, err := resolveModel(args.model)
	if err != nil {
		return imageSpec{}, err
	}
	spec := imageSpec{
		model:   model,
		payload: map[string]any{"model": model, "prompt": prompt, "n": args.n},
		fields:  map[string]string{"model": model, "prompt": prompt, "n": strconv.Itoa(args.n)},
		extra:   map[string]any{"model": model, "size": args.size},
	}
	if isGrokImage(model) {
		res, pixel, note := grokResolution(args.size)
		// new-api drops unknown fields like resolution unless pass_through is on.
		// 6rr honors both resolution=2k and OpenAI size=2048x2048; send both.
		spec.payload["resolution"] = res
		spec.payload["size"] = pixel
		spec.fields["resolution"] = res
		spec.fields["size"] = pixel
		spec.extra["family"] = "grok"
		spec.extra["resolution"] = res
		spec.extra["resolved_size"] = pixel
		if note != "" {
			spec.extra["note"] = note
		}
		if model == "grok-imagine-image-2.0" {
			q := args.quality
			if q == "high" {
				q = "auto"
			}
			spec.payload["quality"] = q
			spec.fields["quality"] = q
			spec.extra["quality"] = q
		}
		return spec, nil
	}
	resolved, err := resolveSize(args.size)
	if err != nil {
		return imageSpec{}, err
	}
	spec.payload["size"] = resolved
	spec.payload["quality"] = args.quality
	spec.fields["size"] = resolved
	spec.fields["quality"] = args.quality
	spec.extra["resolved_size"] = resolved
	spec.extra["quality"] = args.quality
	if strings.HasPrefix(strings.ToLower(model), "gpt-image") {
		spec.extra["family"] = "gpt"
	} else {
		spec.extra["family"] = "other"
	}
	return spec, nil
}

func resultJSON(spec imageSpec, outputs any) map[string]any {
	out := map[string]any{}
	for k, v := range spec.extra {
		out[k] = v
	}
	out["outputs"] = outputs
	return out
}

func requestedBand(size string) string {
	size = normalizeSize(size)
	u := strings.ToUpper(size)
	if u == "1K" || u == "2K" || u == "4K" {
		return u
	}
	if strings.EqualFold(size, "auto") || size == "" {
		return "1K"
	}
	resolved, err := resolveSize(size)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.ToLower(resolved), "x")
	if len(parts) != 2 {
		return ""
	}
	w, e1 := strconv.Atoi(parts[0])
	h, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil {
		return ""
	}
	return qualityBand(w, h)
}

func qualityBand(w, h int) string {
	long := w
	if h > w {
		long = h
	}
	switch {
	case long >= 3000:
		return "4K"
	case long >= 1800:
		return "2K"
	default:
		return "1K"
	}
}

func bandRank(band string) int {
	switch band {
	case "4K":
		return 3
	case "2K":
		return 2
	default:
		return 1
	}
}

func inspectImage(data []byte) (int, int, string) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, ""
	}
	return cfg.Width, cfg.Height, format
}

func attachActual(spec *imageSpec, images [][]byte, asked string) {
	if spec == nil || len(images) == 0 {
		return
	}
	w, h, format := inspectImage(images[0])
	if w < 1 || h < 1 {
		return
	}
	actual := qualityBand(w, h)
	spec.extra["actual_size"] = fmt.Sprintf("%dx%d", w, h)
	spec.extra["actual_format"] = format
	spec.extra["actual_quality"] = actual
	askedBand := requestedBand(asked)
	if askedBand == "" || bandRank(askedBand) <= bandRank(actual) {
		return
	}
	warn := fmt.Sprintf("上游 API 不支持 %s 的 %s 画质（未按请求分辨率出图，实际 %dx%d，相当于 %s）。请改用 %s，或换 GPT 生图模型。", spec.model, askedBand, w, h, actual, actual)
	spec.extra["warning"] = warn
	fmt.Fprintln(os.Stderr, "warning:", warn)
}

func stringifyErrorValue(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		if msg, ok := t["message"]; ok {
			return stringifyErrorValue(msg)
		}
	}
	return ""
}

func extractJSONError(body []byte) string {
	var top map[string]any
	if err := json.Unmarshal(body, &top); err != nil {
		s := strings.TrimSpace(string(body))
		if len(s) > 240 {
			s = s[:240]
		}
		return s
	}
	if msg := stringifyErrorValue(top["error"]); msg != "" {
		return msg
	}
	return stringifyErrorValue(top["message"])
}

func formatAPIError(status int, body []byte) string {
	msg := extractJSONError(body)
	if msg == "" {
		return fmt.Sprintf("上游 API 返回 HTTP %d", status)
	}
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "size") || strings.Contains(lower, "resolution") || strings.Contains(lower, "dimension") || strings.Contains(lower, "4k") || strings.Contains(lower, "2k") || strings.Contains(lower, "画质") {
		return fmt.Sprintf("上游 API 不支持这次请求的画质或尺寸：%s", msg)
	}
	return fmt.Sprintf("上游 API 返回 HTTP %d：%s", status, msg)
}

func validateCommon(args commonArgs) error {
	if _, err := resolveModel(args.model); err != nil {
		return err
	}
	if args.n < 1 || args.n > 10 {
		return errors.New("n must be from 1 to 10")
	}
	if args.maxAttempts < 1 {
		return errors.New("--max-attempts must be at least 1")
	}
	if args.timeout <= 0 {
		return errors.New("--timeout must be positive")
	}
	if args.quality != "low" && args.quality != "medium" && args.quality != "high" && args.quality != "auto" {
		return errors.New("quality must be low, medium, high, or auto")
	}
	if _, err := resolveSize(args.size); err != nil {
		return err
	}
	return nil
}

func promptValue(prompt, promptFile string) (string, error) {
	if (prompt == "") == (promptFile == "") {
		return "", errors.New("provide exactly one of --prompt or --prompt-file")
	}
	if promptFile != "" {
		data, err := os.ReadFile(promptFile)
		if err != nil {
			return "", fmt.Errorf("could not read prompt file: %w", err)
		}
		prompt = string(data)
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", errors.New("prompt must not be empty")
	}
	return prompt, nil
}

func outputPaths(out, outDir, prompt string, n int) []string {
	if out == "" {
		sum := sha256.Sum256([]byte(prompt))
		out = "image-" + hex.EncodeToString(sum[:6]) + ".png"
	}
	if filepath.Dir(out) == "." {
		out = filepath.Join(outDir, out)
	}
	ext := filepath.Ext(out)
	if ext == "" {
		ext = ".png"
		out += ext
	}
	if n == 1 {
		return []string{out}
	}
	base := strings.TrimSuffix(out, ext)
	paths := make([]string, n)
	for i := range paths {
		paths[i] = fmt.Sprintf("%s-%d%s", base, i+1, ext)
	}
	return paths
}

func checkOutputs(paths []string, force bool) error {
	if force {
		return nil
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("refusing to overwrite existing file: %s", path)
		}
	}
	return nil
}

func apiKey() (string, error) {
	key := strings.TrimSpace(os.Getenv("CODEX_API_KEY"))
	if key == "" {
		return "", errors.New("CODEX_API_KEY is not set; set it locally, then retry")
	}
	return key, nil
}

func doRequest(client *http.Client, makeRequest func() (*http.Request, error), maxAttempts int) ([]byte, error) {
	var last string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := makeRequest()
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				return nil, errors.New("could not read API response")
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return body, nil
			}
			last = formatAPIError(resp.StatusCode, body)
			if !retryable[resp.StatusCode] || attempt == maxAttempts {
				return nil, errors.New(last)
			}
		} else {
			last = "API request failed: network error or timeout"
			if attempt == maxAttempts {
				return nil, errors.New(last)
			}
		}
		time.Sleep(time.Duration(750*(1<<(attempt-1))) * time.Millisecond)
	}
	return nil, errors.New(last)
}

func decodeResponse(raw []byte, client *http.Client) ([][]byte, error) {
	var result apiResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, errors.New("API returned invalid JSON")
	}
	if len(result.Data) == 0 {
		return nil, errors.New("API response contains no image data")
	}
	images := make([][]byte, 0, len(result.Data))
	for _, item := range result.Data {
		if item.B64JSON != "" {
			data, err := base64.StdEncoding.DecodeString(item.B64JSON)
			if err != nil {
				return nil, errors.New("API returned invalid base64 image data")
			}
			images = append(images, data)
		} else if item.URL != "" {
			if _, err := url.ParseRequestURI(item.URL); err != nil {
				return nil, errors.New("API returned an invalid image URL")
			}
			resp, err := client.Get(item.URL)
			if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
				if resp != nil {
					resp.Body.Close()
				}
				return nil, errors.New("could not download image URL")
			}
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return nil, errors.New("could not read downloaded image")
			}
			images = append(images, data)
		} else {
			return nil, errors.New("API image entry contains neither b64_json nor url")
		}
	}
	return images, nil
}

func saveImages(images [][]byte, paths []string) ([]string, error) {
	if len(images) != len(paths) {
		return nil, fmt.Errorf("API returned %d image(s), expected %d", len(images), len(paths))
	}
	abs := make([]string, len(paths))
	for i, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, fmt.Errorf("could not create output directory: %w", err)
		}
		if err := os.WriteFile(path, images[i], 0644); err != nil {
			return nil, fmt.Errorf("could not write output: %w", err)
		}
		abs[i], _ = filepath.Abs(path)
	}
	return abs, nil
}

func generate(prompt, out string, args commonArgs) (map[string]any, error) {
	if err := validateCommon(args); err != nil {
		return nil, err
	}
	paths := outputPaths(out, args.outDir, prompt, args.n)
	if err := checkOutputs(paths, args.force); err != nil {
		return nil, err
	}
	ep := endpoint(os.Getenv("CODEX_API_URL"), "generations")
	spec, err := prepareImageRequest(prompt, args)
	if err != nil {
		return nil, err
	}
	if args.dryRun {
		return map[string]any{"dry_run": true, "endpoint": ep, "payload": spec.payload, "outputs": paths}, nil
	}
	key, err := apiKey()
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(spec.payload)
	client := &http.Client{Timeout: args.timeout}
	raw, err := doRequest(client, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, ep, bytes.NewReader(body))
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+key)
			req.Header.Set("Content-Type", "application/json")
		}
		return req, err
	}, args.maxAttempts)
	if err != nil {
		return nil, err
	}
	images, err := decodeResponse(raw, client)
	if err != nil {
		return nil, err
	}
	attachActual(&spec, images, args.size)
	outputs, err := saveImages(images, paths)
	if err != nil {
		return nil, err
	}
	return resultJSON(spec, outputs), nil
}

func edit(prompt string, imagePaths []string, mask, out string, args commonArgs) (map[string]any, error) {
	if err := validateCommon(args); err != nil {
		return nil, err
	}
	for _, path := range append(append([]string{}, imagePaths...), mask) {
		if path == "" {
			continue
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return nil, fmt.Errorf("input file not found: %s", path)
		}
	}
	paths := outputPaths(out, args.outDir, prompt, args.n)
	if err := checkOutputs(paths, args.force); err != nil {
		return nil, err
	}
	ep := endpoint(os.Getenv("CODEX_API_URL"), "edits")
	spec, err := prepareImageRequest(prompt, args)
	if err != nil {
		return nil, err
	}
	if args.dryRun {
		return map[string]any{"dry_run": true, "endpoint": ep, "fields": spec.fields, "images": imagePaths, "mask": mask, "outputs": paths}, nil
	}
	key, err := apiKey()
	if err != nil {
		return nil, err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range spec.fields {
		_ = writer.WriteField(name, value)
	}
	for _, path := range imagePaths {
		part, err := writer.CreateFormFile("image", filepath.Base(path))
		if err != nil {
			return nil, err
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(part, file)
		file.Close()
		if err != nil {
			return nil, err
		}
	}
	if mask != "" {
		part, err := writer.CreateFormFile("mask", filepath.Base(mask))
		if err != nil {
			return nil, err
		}
		file, err := os.Open(mask)
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(part, file)
		file.Close()
		if err != nil {
			return nil, err
		}
	}
	writer.Close()
	contentType := writer.FormDataContentType()
	data := append([]byte(nil), body.Bytes()...)
	client := &http.Client{Timeout: args.timeout}
	raw, err := doRequest(client, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, ep, bytes.NewReader(data))
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+key)
			req.Header.Set("Content-Type", contentType)
		}
		return req, err
	}, args.maxAttempts)
	if err != nil {
		return nil, err
	}
	images, err := decodeResponse(raw, client)
	if err != nil {
		return nil, err
	}
	attachActual(&spec, images, args.size)
	outputs, err := saveImages(images, paths)
	if err != nil {
		return nil, err
	}
	return resultJSON(spec, outputs), nil
}

func printJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}

func parseCommon(fs *flag.FlagSet, argv []string, args *commonArgs) error {
	var timeout float64
	fs.StringVar(&args.model, "model", currentDefaultModel(), "image model (default gpt-image-2.5; say Image 2.6 or any gateway id)")
	fs.StringVar(&args.size, "size", defaultSize, "1K, 2K, 4K, auto, or WIDTHxHEIGHT")
	fs.StringVar(&args.quality, "quality", defaultQuality, "low, medium, high, or auto")
	fs.IntVar(&args.n, "n", 1, "number of variants (1-10)")
	fs.StringVar(&args.outDir, "out-dir", defaultOutDir, "default output directory")
	fs.BoolVar(&args.force, "force", false, "overwrite existing outputs")
	fs.BoolVar(&args.dryRun, "dry-run", false, "validate without network access")
	fs.IntVar(&args.maxAttempts, "max-attempts", 3, "maximum attempts")
	fs.Float64Var(&timeout, "timeout", 150, "request timeout in seconds")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	args.timeout = time.Duration(timeout * float64(time.Second))
	return nil
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func runGenerate(argv []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	var prompt, promptFile, out string
	fs.StringVar(&prompt, "prompt", "", "prompt text")
	fs.StringVar(&promptFile, "prompt-file", "", "UTF-8 prompt file")
	fs.StringVar(&out, "out", "", "output path")
	var args commonArgs
	if err := parseCommon(fs, argv, &args); err != nil {
		return err
	}
	finalPrompt, err := promptValue(prompt, promptFile)
	if err != nil {
		return err
	}
	result, err := generate(finalPrompt, out, args)
	if err == nil {
		printJSON(result)
	}
	return err
}

func runEdit(argv []string) error {
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	var images stringList
	var prompt, promptFile, mask, out string
	fs.Var(&images, "image", "input image; repeat for multiple images")
	fs.StringVar(&mask, "mask", "", "optional PNG mask")
	fs.StringVar(&prompt, "prompt", "", "prompt text")
	fs.StringVar(&promptFile, "prompt-file", "", "UTF-8 prompt file")
	fs.StringVar(&out, "out", "", "output path")
	var args commonArgs
	if err := parseCommon(fs, argv, &args); err != nil {
		return err
	}
	if len(images) == 0 {
		return errors.New("provide at least one --image")
	}
	finalPrompt, err := promptValue(prompt, promptFile)
	if err != nil {
		return err
	}
	result, err := edit(finalPrompt, images, mask, out, args)
	if err == nil {
		printJSON(result)
	}
	return err
}

func runBatch(argv []string) error {
	fs := flag.NewFlagSet("generate-batch", flag.ContinueOnError)
	var input string
	var concurrency int
	var failFast bool
	fs.StringVar(&input, "input", "", "JSONL input path")
	fs.IntVar(&concurrency, "concurrency", 2, "parallel jobs")
	fs.BoolVar(&failFast, "fail-fast", false, "stop scheduling after a failure")
	var args commonArgs
	if err := parseCommon(fs, argv, &args); err != nil {
		return err
	}
	if input == "" || concurrency < 1 {
		return errors.New("--input is required and --concurrency must be at least 1")
	}
	file, err := os.Open(input)
	if err != nil {
		return fmt.Errorf("could not read batch input: %w", err)
	}
	defer file.Close()
	var jobs []batchJob
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var job batchJob
		if err := json.Unmarshal(scanner.Bytes(), &job); err != nil || strings.TrimSpace(job.Prompt) == "" {
			return errors.New("each JSONL line must contain a non-empty prompt")
		}
		jobs = append(jobs, job)
	}
	if err := scanner.Err(); err != nil || len(jobs) == 0 {
		return errors.New("batch input contains no valid jobs")
	}
	type result struct {
		Index int            `json:"index"`
		OK    bool           `json:"ok"`
		Data  map[string]any `json:"data,omitempty"`
		Error string         `json:"error,omitempty"`
	}
	results := make([]result, len(jobs))
	queue := make(chan int)
	var wg sync.WaitGroup
	var failed bool
	var mu sync.Mutex
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range queue {
				job := jobs[index]
				jobArgs := args
				if job.Model != "" {
					jobArgs.model = job.Model
				}
				if job.Size != "" {
					jobArgs.size = job.Size
				}
				if job.Quality != "" {
					jobArgs.quality = job.Quality
				}
				if job.N != 0 {
					jobArgs.n = job.N
				}
				data, err := generate(strings.TrimSpace(job.Prompt), job.Out, jobArgs)
				results[index] = result{Index: index + 1, OK: err == nil, Data: data}
				if err != nil {
					results[index].Error = err.Error()
					mu.Lock()
					failed = true
					mu.Unlock()
				}
			}
		}()
	}
	for index := range jobs {
		mu.Lock()
		stop := failFast && failed
		mu.Unlock()
		if stop {
			break
		}
		queue <- index
	}
	close(queue)
	wg.Wait()
	succeeded, failures := 0, 0
	for _, item := range results {
		if item.Index == 0 {
			continue
		}
		if item.OK {
			succeeded++
		} else {
			failures++
		}
	}
	printJSON(map[string]any{"jobs": results, "succeeded": succeeded, "failed": failures})
	if failures > 0 {
		return errors.New("one or more batch jobs failed")
	}
	return nil
}

func isProbablyImageModel(id string) bool {
	key := strings.ToLower(id)
	if isProbablyVideoModel(id) {
		return false
	}
	return strings.Contains(key, "image") || strings.Contains(key, "dall") || strings.Contains(key, "flux") || strings.Contains(key, "imagen") || strings.Contains(key, "grok-imagine")
}

func isProbablyVideoModel(id string) bool {
	key := strings.ToLower(id)
	return strings.Contains(key, "video") || strings.Contains(key, "sora") || strings.Contains(key, "kling") || strings.Contains(key, "runway") || strings.Contains(key, "luma") || strings.Contains(key, "jimeng") || strings.Contains(key, "即梦")
}

func jsonString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := value[key].(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func asObject(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func videoTaskID(payload map[string]any) string {
	if id := jsonString(payload, "id", "task_id", "request_id"); id != "" {
		return id
	}
	if nested := asObject(payload["data"]); nested != nil {
		return jsonString(nested, "id", "task_id", "request_id")
	}
	return ""
}

func videoStatus(payload map[string]any) string {
	status := jsonString(payload, "status", "task_status")
	if status == "" {
		if nested := asObject(payload["data"]); nested != nil {
			status = jsonString(nested, "status", "task_status")
		}
	}
	return strings.ToLower(status)
}

func videoResultURL(payload map[string]any) string {
	if url := jsonString(payload, "url", "result_url", "video_url"); url != "" {
		return url
	}
	if nested := asObject(payload["data"]); nested != nil {
		return jsonString(nested, "url", "result_url", "video_url")
	}
	return ""
}

func videoErrorMessage(payload map[string]any) string {
	if msg := stringifyErrorValue(payload["error"]); msg != "" {
		return msg
	}
	if nested := asObject(payload["data"]); nested != nil {
		if msg := stringifyErrorValue(nested["error"]); msg != "" {
			return msg
		}
		if msg := jsonString(nested, "fail_reason", "message"); msg != "" {
			return msg
		}
	}
	return jsonString(payload, "fail_reason", "message")
}

func outputVideoPath(out, outDir, prompt string) string {
	if out == "" {
		sum := sha256.Sum256([]byte(prompt))
		out = "video-" + hex.EncodeToString(sum[:6]) + ".mp4"
	}
	if filepath.Dir(out) == "." {
		out = filepath.Join(outDir, out)
	}
	if filepath.Ext(out) == "" {
		out += ".mp4"
	}
	return out
}

func postJSON(client *http.Client, endpoint, key string, payload map[string]any, attempts int) (map[string]any, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	raw, err := doRequest(client, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+key)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")
		}
		return req, err
	}, attempts)
	if err != nil {
		return nil, raw, err
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, raw, errors.New("API returned invalid JSON")
	}
	return parsed, raw, nil
}

func getJSON(client *http.Client, endpoint, key string, attempts int) (map[string]any, error) {
	raw, err := doRequest(client, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+key)
			req.Header.Set("Accept", "application/json")
		}
		return req, err
	}, attempts)
	if err != nil {
		return nil, err
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, errors.New("API returned invalid JSON")
	}
	return parsed, nil
}

func downloadVideo(client *http.Client, endpoint, key, dest string) error {
	raw, err := doRequest(client, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		return req, err
	}, 3)
	if err != nil {
		return err
	}
	if len(raw) < 32 || raw[0] == '{' || raw[0] == '[' {
		return errors.New("video content endpoint did not return a video file")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("could not create output directory: %w", err)
	}
	if err := os.WriteFile(dest, raw, 0644); err != nil {
		return fmt.Errorf("could not write output: %w", err)
	}
	return nil
}

func downloadURL(client *http.Client, source, key, dest string) error {
	req, err := http.NewRequest(http.MethodGet, source, nil)
	if err != nil {
		return errors.New("invalid video URL")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("could not download video URL")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("could not download video URL: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.New("could not read downloaded video")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("could not create output directory: %w", err)
	}
	return os.WriteFile(dest, data, 0644)
}

func submitVideo(client *http.Client, root, key string, payload map[string]any, attempts int) (map[string]any, string, error) {
	parsed, _, err := postJSON(client, root+"/videos", key, payload, attempts)
	if err == nil {
		return parsed, root + "/videos", nil
	}
	fallback, _, fallbackErr := postJSON(client, root+"/video/generations", key, payload, attempts)
	if fallbackErr == nil {
		return fallback, root + "/video/generations", nil
	}
	return nil, "", err
}

func pollVideo(client *http.Client, root, key, taskID string, timeout time.Duration) (map[string]any, error) {
	deadline := time.Now().Add(timeout)
	paths := []string{root + "/videos/" + taskID, root + "/video/generations/" + taskID}
	var last map[string]any
	for time.Now().Before(deadline) {
		var err error
		for _, path := range paths {
			last, err = getJSON(client, path, key, 2)
			if err == nil {
				break
			}
		}
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		switch videoStatus(last) {
		case "completed", "success", "succeed", "succeeded":
			return last, nil
		case "failed", "failure", "fail":
			if msg := videoErrorMessage(last); msg != "" {
				return last, errors.New(msg)
			}
			return last, errors.New("video generation failed")
		}
		time.Sleep(5 * time.Second)
	}
	return last, errors.New("video generation timed out")
}

func generateVideo(prompt, out string, model, size string, seconds int, force, dryRun bool, timeout time.Duration, attempts int) (map[string]any, error) {
	resolvedModel, err := resolveVideoModel(model)
	if err != nil {
		return nil, err
	}
	pixels, band, note, err := resolveVideoSize(size)
	if err != nil {
		return nil, err
	}
	if seconds < 1 {
		seconds = defaultVideoSecs
	}
	dest := outputVideoPath(out, defaultVideoOutDir, prompt)
	if err := checkOutputs([]string{dest}, force); err != nil {
		return nil, err
	}
	payload := map[string]any{
		"model":   resolvedModel,
		"prompt":  prompt,
		"size":    pixels,
		"seconds": strconv.Itoa(seconds),
	}
	result := map[string]any{
		"model":         resolvedModel,
		"size":          band,
		"resolved_size": pixels,
		"seconds":       seconds,
		"outputs":       []string{dest},
	}
	if note != "" {
		result["note"] = note
	}
	if dryRun {
		result["dry_run"] = true
		result["payload"] = payload
		return result, nil
	}
	key, err := apiKey()
	if err != nil {
		return nil, err
	}
	root := apiRoot(os.Getenv("CODEX_API_URL"))
	client := &http.Client{Timeout: 120 * time.Second}
	submitted, endpoint, err := submitVideo(client, root, key, payload, attempts)
	if err != nil {
		return nil, err
	}
	taskID := videoTaskID(submitted)
	if taskID == "" {
		return nil, errors.New("API did not return a video task id")
	}
	result["endpoint"] = endpoint
	result["task_id"] = taskID
	status, err := pollVideo(client, root, key, taskID, timeout)
	if err != nil {
		return result, err
	}
	abs, err := filepath.Abs(dest)
	if err != nil {
		abs = dest
	}
	contentURL := root + "/videos/" + taskID + "/content"
	if err := downloadVideo(client, contentURL, key, dest); err != nil {
		source := videoResultURL(status)
		if source == "" {
			return result, err
		}
		if err := downloadURL(client, source, key, dest); err != nil {
			return result, err
		}
	}
	result["status"] = videoStatus(status)
	result["outputs"] = []string{abs}
	return result, nil
}

func runVideo(argv []string) error {
	fs := flag.NewFlagSet("video", flag.ContinueOnError)
	var prompt, promptFile, out, model, size string
	var seconds int
	var force, dryRun bool
	var timeoutSec float64
	var attempts int
	fs.StringVar(&prompt, "prompt", "", "prompt text")
	fs.StringVar(&promptFile, "prompt-file", "", "UTF-8 prompt file")
	fs.StringVar(&out, "out", "", "output path")
	fs.StringVar(&model, "model", "", "video model; default grok-imagine-video")
	fs.StringVar(&size, "size", defaultSize, "1K, 2K, 4K, or WIDTHxHEIGHT")
	fs.IntVar(&seconds, "seconds", defaultVideoSecs, "clip length in seconds")
	fs.BoolVar(&force, "force", false, "overwrite existing outputs")
	fs.BoolVar(&dryRun, "dry-run", false, "validate without network access")
	fs.Float64Var(&timeoutSec, "timeout", 600, "wait timeout in seconds")
	fs.IntVar(&attempts, "max-attempts", 3, "maximum submit attempts")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	finalPrompt, err := promptValue(prompt, promptFile)
	if err != nil {
		return err
	}
	result, err := generateVideo(finalPrompt, out, model, size, seconds, force, dryRun, time.Duration(timeoutSec*float64(time.Second)), attempts)
	if err == nil {
		printJSON(result)
	}
	return err
}

func runModels(argv []string) error {
	fs := flag.NewFlagSet("models", flag.ContinueOnError)
	var all bool
	fs.BoolVar(&all, "all", false, "list every gateway model, not only image and video models")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	out := map[string]any{
		"defaults": map[string]any{
			"image": currentDefaultModel(),
			"video": currentDefaultVideoModel(),
			"size":  defaultSize,
		},
		"note": "Codex Image 2.5. Unspecified image uses gpt-image-2.5. Say 2.6, G2.6, or a later version to switch. Unspecified video uses grok-imagine-video. Unspecified size uses 1K.",
	}
	key, err := apiKey()
	if err != nil {
		out["gateway_error"] = err.Error()
		printJSON(out)
		return nil
	}
	ep := apiRoot(os.Getenv("CODEX_API_URL")) + "/models"
	client := &http.Client{Timeout: 30 * time.Second}
	raw, err := doRequest(client, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, ep, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+key)
			req.Header.Set("Accept", "application/json")
		}
		return req, err
	}, 3)
	if err != nil {
		out["gateway_error"] = err.Error()
		printJSON(out)
		return nil
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		out["gateway_error"] = "could not parse /v1/models"
		printJSON(out)
		return nil
	}
	images, videos, others := []string{}, []string{}, []string{}
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		switch {
		case isProbablyVideoModel(id):
			videos = append(videos, id)
		case isProbablyImageModel(id):
			images = append(images, id)
		case all:
			others = append(others, id)
		}
	}
	out["images"] = images
	out["videos"] = videos
	if all {
		out["other"] = others
	}
	printJSON(out)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "Codex Image")
	fmt.Fprintln(os.Stderr, "Usage: codex-image2-5 <generate|generate-batch|edit|video|models> [options]")
	fmt.Fprintln(os.Stderr, "Image default: --model gpt-image-2.5  --size 1K  --quality auto")
	fmt.Fprintln(os.Stderr, "Video default: --model grok-imagine-video  --size 1K  --seconds 8")
	fmt.Fprintln(os.Stderr, "Named versions: Image 2, 2.5, 2.6, G2.6, 2.7... → gpt-image-{version}")
	fmt.Fprintln(os.Stderr, "GPT variants: 闪焰/flare  日耀/sunburst  latest/最新")
	fmt.Fprintln(os.Stderr, "Grok image: grok-imagine  grok-imagine-image-2.0  grok-imagine-image-quality")
	fmt.Fprintln(os.Stderr, "Video: grok-imagine-video  sora-2  or any gateway video id")
	fmt.Fprintln(os.Stderr, "Ask models to list what the current gateway actually has.")
	fmt.Fprintln(os.Stderr, "GPT size: 1K=1024x1024  2K=2048x2048  4K=3840x2160")
	fmt.Fprintln(os.Stderr, "Video size: 1K=1280x720  2K=1920x1080  4K requests 1080p")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "generate":
		err = runGenerate(os.Args[2:])
	case "generate-batch":
		err = runBatch(os.Args[2:])
	case "edit":
		err = runEdit(os.Args[2:])
	case "video":
		err = runVideo(os.Args[2:])
	case "models":
		err = runModels(os.Args[2:])
	case "--help", "-h", "help":
		usage()
		return
	default:
		err = fmt.Errorf("unknown command: %s", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}
