package api

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/akutz/sortfold"
	"github.com/briandowns/spinner"
	"github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/natefinch/atomic"
)

type Item struct {
	Time   string
	Title  string
	Type   string
	Number int
	Url    string
	Author string
}

type Cve struct {
	Id       string
	Title    string
	Url      string
	Versions []string
}

func Changelog(milestone string, version string) error {
	s := spinner.New(spinner.CharSets[11], 120*time.Millisecond)
	s.Start()

	repo, err := repository.Current()
	if err != nil {
		s.Stop()
		return err
	}

	slug := fmt.Sprintf("%s/%s", repo.Owner, repo.Name)

	items, err := getItems(slug, milestone)
	if err != nil {
		s.Stop()
		return err
	}

	cves, err := getCves(slug)
	if err != nil {
		s.Stop()
		return err
	}

	r := strings.NewReader(getContent(items, cves, slug, version))

	if err := atomic.WriteFile("./CHANGELOG.md", r); err != nil {
		s.Stop()
		return err
	}

	s.Stop()
	fmt.Println("The CHANGELOG.md file has been updated.")
	return nil
}

func getItems(repo string, milestone string) ([]Item, error) {
	tags, err := getTags(milestone)
	if err != nil {
		return nil, err
	}

	features, err := search(repo, milestone, "feature")
	if err != nil {
		return nil, err
	}

	issues, err := search(repo, milestone, "bug")
	if err != nil {
		return nil, err
	}

	var items []Item
	items = append(items, tags...)
	items = append(items, features...)
	items = append(items, issues...)

	sort.Slice(items, func(i, j int) bool {
		return items[i].Time > items[j].Time // reverse sort
	})

	return items, nil
}

func getTags(milestone string) ([]Item, error) {
	args := []string{
		"tag",
		"--list", milestone + ".*",
		"--sort", "-creatordate",
		"--format", "%(creatordate:format-local:%Y-%m-%dT%H:%M:%SZ),%(refname:short)",
	}

	cmd := exec.Command("git", args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "TZ=UTC0")

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var items []Item
	r := csv.NewReader(bytes.NewReader(out))

	for {
		fields, err := r.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if len(fields) < 2 {
			continue
		}

		var item Item
		item.Time = fields[0]
		item.Title = fields[1]
		item.Type = "tag"

		items = append(items, item)
	}

	return items, nil
}

func search(repo string, milestone string, label string) ([]Item, error) {
	args := []string{
		"search", "prs",
		"--json", "number,title,author,url,closedAt",
		"--repo", repo,
		"--milestone", milestone,
		"--label", label,
		"--limit", "1000",
		"--merged",
	}

	data, _, err := gh.Exec(args...)
	if err != nil {
		return nil, err
	}

	var r []struct {
		Time   string `json:"closedAt"`
		Title  string `json:"title"`
		Number int    `json:"number"`
		Url    string `json:"url"`
		Author struct {
			Login string `json:"login"`
		}
	}

	if err := json.Unmarshal(data.Bytes(), &r); err != nil {
		return nil, err
	}

	var items []Item

	for i := 0; i < len(r); i++ {
		var item Item
		item.Time = r[i].Time
		item.Title = r[i].Title
		item.Type = label
		item.Number = r[i].Number
		item.Url = r[i].Url
		item.Author = r[i].Author.Login

		items = append(items, item)
	}

	return items, nil
}

func getCves(repo string) ([]Cve, error) {
	args := []string{
		"api",
		"-X", "GET",
		"-F", "state=published",
		"repos/" + repo + "/security-advisories",
	}

	data, _, err := gh.Exec(args...)
	if err != nil {
		return nil, err
	}

	var s []struct {
		Title           string `json:"summary"`
		Cve             string `json:"cve_id"`
		Url             string `json:"html_url"`
		Vulnerabilities []struct {
			PatchedVersions string `json:"patched_versions"`
		}
	}

	if err := json.Unmarshal(data.Bytes(), &s); err != nil {
		return nil, err
	}

	var cves []Cve

	for i := 0; i < len(s); i++ {
		var cve Cve
		cve.Id = s[i].Cve
		cve.Title = s[i].Title
		cve.Url = s[i].Url

		cve.Versions = nil
		if len(s[i].Vulnerabilities) > 0 {
			cve.Versions = strings.Split(s[i].Vulnerabilities[0].PatchedVersions, ", ")
		}

		cves = append(cves, cve)
	}

	return cves, nil
}

func getContent(items []Item, cves []Cve, repo string, version string) string {
	var tags []string
	var features []Item
	var issues []Item
	var security []Cve
	var advisories []string

	users := make(map[string]string)
	prs := make(map[int]string)
	url := "https://github.com/" + repo

	if version != "Unreleased" {
		tags = append(tags, fmt.Sprintf("[%s]: %s/releases/tag/%[1]s\n", version, url))
	}

	var content strings.Builder
	content.WriteString("# Changelog\n\nThis project adheres to [Semantic Versioning].\n\n")
	fmt.Fprintf(&content, "## [%s] (%s)\n", version, time.Now().Format("2006-01-02"))

	for i := range items {
		if items[i].Type == "tag" {
			for j := range cves {
				for _, val := range cves[j].Versions {
					if val == items[i].Title {
						security = append(security, cves[j])
						advisories = append(advisories, fmt.Sprintf("[%s]: %s\n", cves[j].Id, cves[j].Url))
					}
				}
			}

			tagTime := items[i].Time
			if len(tagTime) >= 10 {
				tagTime = tagTime[0:10]
			}

			content.WriteString(addSection(&features, &issues))
			fmt.Fprintf(&content, "\n## [%s] (%s)\n", items[i].Title, tagTime)
			content.WriteString(addSecurity(&security))

			tags = append(tags, fmt.Sprintf("[%s]: %s/releases/tag/%[1]s\n", items[i].Title, url))
		} else {
			switch items[i].Type {
			case "feature":
				features = append(features, items[i])
			case "bug":
				issues = append(issues, items[i])
			}

			users[items[i].Author] = fmt.Sprintf("[%s]: https://github.com/%[1]s\n", items[i].Author)
			prs[items[i].Number] = fmt.Sprintf("[#%d]: %s/pull/%[1]d\n", items[i].Number, url)
		}
	}

	content.WriteString(addSecurity(&security))
	content.WriteString(addSection(&features, &issues))
	content.WriteString("\n[Semantic Versioning]: https://semver.org/spec/v2.0.0.html\n")

	for i := 0; i < len(tags); i++ {
		content.WriteString(tags[i])
	}

	for i := 0; i < len(advisories); i++ {
		content.WriteString(advisories[i])
	}

	for _, k := range getUserKeys(users) {
		content.WriteString(users[k])
	}

	for _, k := range getPrKeys(prs) {
		content.WriteString(prs[k])
	}

	return content.String()
}

func addSection(features *[]Item, issues *[]Item) string {
	var r strings.Builder

	if len(*features) > 0 {
		r.WriteString("\n**New features:**\n\n")

		for _, f := range *features {
			fmt.Fprintf(&r, "- [#%d] %s ([%s])\n", f.Number, f.Title, f.Author)
		}

		*features = nil
	}

	if len(*issues) > 0 {
		r.WriteString("\n**Fixed issues:**\n\n")

		for _, i := range *issues {
			fmt.Fprintf(&r, "- [#%d] %s ([%s])\n", i.Number, i.Title, i.Author)
		}

		*issues = nil
	}

	return r.String()
}

func addSecurity(cves *[]Cve) string {
	var r strings.Builder

	if len(*cves) > 0 {
		r.WriteString("\n**Security fixes:**\n\n")

		for _, c := range *cves {
			fmt.Fprintf(&r, "- [%s]: %s\n", c.Id, c.Title)
		}

		*cves = nil
	}

	return r.String()
}

func getUserKeys(users map[string]string) []string {
	keys := make([]string, 0, len(users))

	for k := range users {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		return sortfold.CompareFold(keys[i], keys[j]) < 0
	})

	return keys
}

func getPrKeys(prs map[int]string) []int {
	keys := make([]int, 0, len(prs))

	for k := range prs {
		keys = append(keys, k)
	}

	sort.Ints(keys)

	return keys
}
