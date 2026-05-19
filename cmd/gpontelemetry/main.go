package main

import (
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type metric struct {
	sum       float64
	count     int
	min, max  float64
	hasValues bool
}

type sample struct {
	t       time.Time
	n       int
	ok, bad int
	m       map[string]metric
}

var tsRE = regexp.MustCompile(`([A-Z][a-z]{2})/(\d{2})/(\d{4}) (\d{2}):(\d{2}):(\d{2})`)

const rawKeepSamples = 288

var months = map[string]time.Month{
	"Jan": time.January, "Feb": time.February, "Mar": time.March,
	"Apr": time.April, "May": time.May, "Jun": time.June,
	"Jul": time.July, "Aug": time.August, "Sep": time.September,
	"Oct": time.October, "Nov": time.November, "Dec": time.December,
}

func main() {
	root := envOr("GPON_LOG_ROOT", envOr("GPON_ROOT", "/var/lib/gpontelemetry"))
	mode := "all"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	if mode == "sample" {
		line, err := pollLine()
		fmt.Println(line)
		if err != nil {
			os.Exit(0)
		}
		return
	}

	if mode == "poll" || mode == "poll-only" {
		if err := pollHTTP(root); err != nil {
			log.Printf("poll: %v", err)
		}
		if mode == "poll-only" {
			return
		}
		mode = "all"
	}

	plans := map[string]struct {
		src, dst, prefix string
		bucket, keep     time.Duration
	}{
		"week":  {"gpon.0.txt", "gpon-week.0.txt", "GPONWEEK", 30 * time.Minute, 7 * 24 * time.Hour},
		"month": {"gpon-week.0.txt", "gpon-month.0.txt", "GPONMONTH", 2 * time.Hour, 31 * 24 * time.Hour},
		"year":  {"gpon-month.0.txt", "gpon-year.0.txt", "GPONYEAR", 24 * time.Hour, 366 * 24 * time.Hour},
	}

	order := []string{mode}
	if mode == "all" {
		order = []string{"week", "month", "year"}
	}
	for _, name := range order {
		p, ok := plans[name]
		if !ok {
			log.Fatalf("unknown rollup %q", name)
		}
		if err := rollup(root, p.src, p.dst, p.prefix, p.bucket, p.keep); err != nil {
			log.Printf("%s: %v", name, err)
		}
	}
}

func pollHTTP(root string) error {
	now := time.Now().In(manila())
	line, err := pollLine()
	if err := appendRaw(root, formatTime(now)+" "+line); err != nil {
		return err
	}
	log.Printf("%s", line)
	return err
}

func pollLine() (string, error) {
	status, err := fetchPONStatus()
	if err != nil {
		return fmt.Sprintf("GPONRAW UNHEALTHY | fetch failed: %s", sanitize(err.Error())), err
	}

	state := status["state"]
	level := "UNHEALTHY"
	if state == "O5" {
		level = "OK"
	}
	return fmt.Sprintf("GPONRAW %s | state=Operation State(%s) temp=%s rx=%s tx=%s v=%s bias=%s",
		level, state, status["temp"], status["rx"], status["tx"], status["v"], status["bias"]), nil
}

func fetchPONStatus() (map[string]string, error) {
	base := stickURL()
	user := envOr("GPON_USER", "admin")
	pass := envOr("GPON_PASS", "admin")
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}

	form := url.Values{
		"challenge":  {""},
		"username":   {user},
		"password":   {pass},
		"save":       {"Login"},
		"submit-url": {"/admin/login.asp"},
	}
	resp, err := client.PostForm(base+"/boaform/admin/formLogin", form)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("login HTTP %d", resp.StatusCode)
	}
	defer logout(client, base)

	resp, err = client.Get(base + "/status_pon.asp")
	if err != nil {
		return nil, fmt.Errorf("status fetch failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status_pon.asp HTTP %d", resp.StatusCode)
	}
	html := string(body)
	status := map[string]string{
		"temp":  firstFloat(pageValue(html, "Temperature")),
		"v":     firstFloat(pageValue(html, "Voltage")),
		"tx":    firstFloat(pageValue(html, "Tx Power")),
		"rx":    firstFloat(pageValue(html, "Rx Power")),
		"bias":  firstFloat(pageValue(html, "Bias Current")),
		"state": strings.TrimSpace(pageValue(html, "ONU State")),
	}
	if status["state"] == "" {
		return nil, fmt.Errorf("ONU State not found in status_pon.asp")
	}
	for _, k := range []string{"temp", "v", "tx", "rx", "bias"} {
		if status[k] == "" {
			return nil, fmt.Errorf("%s not found in status_pon.asp", k)
		}
	}
	return status, nil
}

func logout(client *http.Client, base string) {
	form := url.Values{
		"save":       {"Logout"},
		"submit-url": {"/admin/logout.asp"},
	}
	resp, err := client.PostForm(base+"/boaform/admin/formLogout", form)
	if err != nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func stickURL() string {
	if v := os.Getenv("GPON_STICK_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	host := envOr("GPON_HOST", "192.168.1.1")
	if strings.Contains(host, "://") {
		return strings.TrimRight(host, "/")
	}
	return "http://" + strings.TrimRight(host, "/")
}

func pageValue(html, label string) string {
	re := regexp.MustCompile(`(?is)<b>\s*` + regexp.QuoteMeta(label) + `\s*</b>\s*</td>\s*<td[^>]*>\s*<font[^>]*>\s*([^<\r\n]+)`)
	m := re.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func firstFloat(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	if _, err := strconv.ParseFloat(fields[0], 64); err != nil {
		return ""
	}
	return fields[0]
}

func appendRaw(root, line string) error {
	path := filepath.Join(root, "gpon.0.txt")
	old, _ := os.ReadFile(path)
	text := strings.TrimRight(string(old), "\r\n")
	if text != "" {
		text += "\n"
	}
	text += line + "\n"

	rows := splitRecords(text)
	if len(rows) > rawKeepSamples {
		rows = rows[len(rows)-rawKeepSamples:]
		text = strings.Join(rows, "\n") + "\n"
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0644); err != nil {
		return fmt.Errorf("write raw tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace raw log: %w", err)
	}
	return nil
}

func splitRecords(text string) []string {
	matches := tsRE.FindAllStringSubmatchIndex(text, -1)
	rows := make([]string, 0, len(matches))
	for i, m := range matches {
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		rec := strings.TrimSpace(text[m[0]:end])
		if rec != "" {
			rows = append(rows, rec)
		}
	}
	return rows
}

func rollup(root, srcName, dstName, prefix string, bucket, keep time.Duration) error {
	srcPath := filepath.Join(root, srcName)
	srcText, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", srcName, err)
	}
	srcSamples := parseLog(string(srcText))
	if len(srcSamples) == 0 {
		return nil
	}

	latest := srcSamples[len(srcSamples)-1].t
	buckets := map[time.Time]sample{}
	for _, s := range srcSamples {
		b := floorTime(s.t, bucket)
		if b.Add(bucket).After(latest) {
			continue
		}
		a := buckets[b]
		if a.n == 0 {
			a = sample{t: b, m: map[string]metric{}}
		}
		a.add(s)
		buckets[b] = a
	}
	if len(buckets) == 0 {
		return nil
	}

	dstPath := filepath.Join(root, dstName)
	if oldText, err := os.ReadFile(dstPath); err == nil {
		for _, s := range parseLog(string(oldText)) {
			if s.t.After(latest) {
				latest = s.t
			}
			buckets[s.t] = s
		}
	}

	cutoff := latest.Add(-keep)
	rows := make([]sample, 0, len(buckets))
	for _, s := range buckets {
		if !s.t.Before(cutoff) {
			rows = append(rows, s)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].t.Before(rows[j].t) })

	var out strings.Builder
	for _, s := range rows {
		out.WriteString(formatSample(prefix, s))
		out.WriteByte('\n')
	}
	tmp := dstPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(out.String()), 0644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(tmp), err)
	}
	if err := os.Rename(tmp, dstPath); err != nil {
		return fmt.Errorf("rename %s: %w", filepath.Base(tmp), err)
	}
	log.Printf("%s: wrote %d rows to %s", prefix, len(rows), dstName)
	return nil
}

func parseLog(text string) []sample {
	matches := tsRE.FindAllStringSubmatchIndex(text, -1)
	rows := make([]sample, 0, len(matches))
	for i, m := range matches {
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		rec := text[m[0]:end]
		t, ok := parseTimestamp(text, m)
		if !ok {
			continue
		}
		s := parseSample(t, rec)
		if s.n > 0 {
			rows = append(rows, s)
		}
	}
	return rows
}

func parseTimestamp(text string, m []int) (time.Time, bool) {
	part := func(n int) string { return text[m[2*n]:m[2*n+1]] }
	month, ok := months[part(1)]
	if !ok {
		return time.Time{}, false
	}
	day, _ := strconv.Atoi(part(2))
	year, _ := strconv.Atoi(part(3))
	hour, _ := strconv.Atoi(part(4))
	min, _ := strconv.Atoi(part(5))
	sec, _ := strconv.Atoi(part(6))
	return time.Date(year, month, day, hour, min, sec, 0, time.UTC), true
}

func parseSample(t time.Time, rec string) sample {
	s := sample{t: t, n: intField(rec, "n", 1), m: map[string]metric{}}
	if bad, ok := maybeIntField(rec, "bad"); ok {
		s.bad = bad
		s.ok = intField(rec, "ok", s.n-bad)
	} else if strings.Contains(rec, "GPONRAW OK") || strings.Contains(rec, "GPON OK") {
		s.ok = 1
	} else if strings.Contains(rec, "GPONRAW UNHEALTHY") || strings.Contains(rec, "GPON UNHEALTHY") {
		s.bad = 1
	} else if strings.Contains(rec, " OK ") {
		s.ok = s.n
	} else if strings.Contains(rec, " UNHEALTHY ") {
		s.bad = s.n
	} else {
		return sample{}
	}

	for _, k := range []string{"rx", "tx", "temp", "v", "bias"} {
		avg, ok := floatField(rec, k+"_avg")
		if !ok {
			avg, ok = floatField(rec, k)
		}
		if !ok {
			continue
		}
		min := avg
		if v, ok := floatField(rec, k+"_min"); ok {
			min = v
		}
		max := avg
		if v, ok := floatField(rec, k+"_max"); ok {
			max = v
		}
		s.m[k] = metric{
			sum:       avg * float64(s.n),
			count:     s.n,
			min:       min,
			max:       max,
			hasValues: true,
		}
	}
	return s
}

func (s *sample) add(o sample) {
	s.n += o.n
	s.ok += o.ok
	s.bad += o.bad
	for k, om := range o.m {
		m := s.m[k]
		if !m.hasValues {
			m = metric{min: math.Inf(1), max: math.Inf(-1), hasValues: true}
		}
		m.sum += om.sum
		m.count += om.count
		if om.min < m.min {
			m.min = om.min
		}
		if om.max > m.max {
			m.max = om.max
		}
		s.m[k] = m
	}
}

func formatSample(prefix string, s sample) string {
	status := "OK"
	if s.bad > 0 {
		status = "UNHEALTHY"
	}
	parts := []string{
		formatTime(s.t),
		prefix,
		status,
		"|",
		fmt.Sprintf("n=%d", s.n),
		fmt.Sprintf("ok=%d", s.ok),
		fmt.Sprintf("bad=%d", s.bad),
	}
	for _, k := range []string{"rx", "tx", "temp", "v", "bias"} {
		m, ok := s.m[k]
		if !ok || m.count == 0 {
			continue
		}
		parts = append(parts,
			fmt.Sprintf("%s_avg=%.2f", k, m.sum/float64(m.count)),
			fmt.Sprintf("%s_min=%.2f", k, m.min),
			fmt.Sprintf("%s_max=%.2f", k, m.max),
		)
	}
	return strings.Join(parts, " ")
}

func floorTime(t time.Time, d time.Duration) time.Time {
	return time.Unix(t.Unix()-t.Unix()%int64(d/time.Second), 0).UTC()
}

func formatTime(t time.Time) string {
	return t.Format("Jan/02/2006 15:04:05")
}

func manila() *time.Location {
	return time.FixedZone("Asia/Manila", 8*60*60)
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func floatField(rec, key string) (float64, bool) {
	v, ok := field(rec, key)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	return f, err == nil
}

func intField(rec, key string, def int) int {
	v, ok := maybeIntField(rec, key)
	if !ok {
		return def
	}
	return v
}

func maybeIntField(rec, key string) (int, bool) {
	raw, ok := field(rec, key)
	if !ok {
		return 0, false
	}
	v, err := strconv.Atoi(raw)
	return v, err == nil
}

func field(rec, key string) (string, bool) {
	pat := key + "="
	p := strings.Index(rec, pat)
	if p < 0 {
		return "", false
	}
	rest := rec[p+len(pat):]
	end := strings.IndexAny(rest, " \r\n")
	if end < 0 {
		end = len(rest)
	}
	return rest[:end], true
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
