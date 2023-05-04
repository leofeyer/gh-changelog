package api

import (
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
	"github.com/cli/go-gh"
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

func Changelog(milestone string, version string) error {
	s := spinner.New(spinner.CharSets[11], 120*time.Millisecond)
	s.Start()

	repo, err := gh.CurrentRepository()
	if err != nil {
		s.Stop()
		return err
	}

	items, err := getItems(milestone, repo.Owner(), repo.Name())
	if err != nil {
		s.Stop()
		return err
	}

	r := strings.NewReader(getContent(items, repo.Owner(), repo.Name(), version))
	atomic.WriteFile("./CHANGELOG.md", r)

	s.Stop()
	fmt.Println("The CHANGELOG.md file has been updated.")
	return nil
}

func getItems(milestone string, owner string, repo string) ([]Item, error) {
	tags, err := getTags(milestone)
	if err != nil {
		return nil, err
	}

	features, err := search(milestone, "feature", owner, repo)
	if err != nil {
		return nil, err
	}

	issues, err := search(milestone, "bug", owner, repo)
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
	r := csv.NewReader(strings.NewReader(fmt.Sprintf("%s", out)))

	for {
		fields, err := r.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		var item Item
		item.Time = fields[0]
		item.Title = fields[1]
		item.Type = "tag"

		items = append(items, item)
	}

	return items, nil
}

func search(milestone string, label string, owner string, repo string) ([]Item, error) {
	args := []string{
		"search", "prs",
		"--json", "number,title,author,url,closedAt",
		"--owner", owner,
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

	type PullRequest struct {
		Time   string `json:"closedAt"`
		Title  string `json:"title"`
		Number int    `json:"number"`
		Url    string `json:"url"`
		Author struct {
			Login string `json:"login"`
		}
	}

	var r []PullRequest

	err = json.Unmarshal(data.Bytes(), &r)
	if err != nil {
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

func getContent(items []Item, owner string, repo string, version string) string {
	var tags []string
	var features []Item
	var issues []Item

	users := make(map[string]string)
	prs := make(map[int]string)
	url := "https://github.com/" + owner + "/" + repo

	if version != "Unreleased" {
		tags = append(tags, fmt.Sprintf("[%s]: %s/releases/tag/%[1]s\n", version, url))
	}

	content := "# Changelog\n\nThis project adheres to [Semantic Versioning].\n\n"
	content += fmt.Sprintf("## [%s] (%s)\n", version, time.Now().Format("2006-01-02"))

	for i := 0; i < len(items); i++ {
		if items[i].Type == "tag" {
			content += addSection(&features, &issues)
			content += fmt.Sprintf("\n## [%s] (%s)\n", items[i].Title, items[i].Time[0:10])

			tags = append(tags, fmt.Sprintf("[%s]: %s/releases/tag/%[1]s\n", items[i].Title, url))
		} else {
			if items[i].Type == "feature" {
				features = append(features, items[i])
			} else if items[i].Type == "bug" {
				issues = append(issues, items[i])
			}

			users[items[i].Author] = fmt.Sprintf("[%s]: https://github.com/%[1]s\n", items[i].Author)
			prs[items[i].Number] = fmt.Sprintf("[#%d]: %s/pull/%[1]d\n", items[i].Number, url)
		}
	}

	content += addSection(&features, &issues)
	content += "\n[Semantic Versioning]: https://semver.org/spec/v2.0.0.html\n"

	for i := 0; i < len(tags); i++ {
		content += tags[i]
	}

	for _, k := range getUserKeys(users) {
		content += users[k]
	}

	for _, k := range getPrKeys(prs) {
		content += prs[k]
	}

	return content
}

func addSection(features *[]Item, issues *[]Item) string {
	r := ""

	if len(*features) > 0 {
		r += "\n**New features:**\n\n"

		for _, f := range *features {
			r += fmt.Sprintf("- [#%d] %s ([%s])\n", f.Number, f.Title, f.Author)
		}

		*features = nil
	}

	if len(*issues) > 0 {
		r += "\n**Fixed issues:**\n\n"

		for _, i := range *issues {
			r += fmt.Sprintf("- [#%d] %s ([%s])\n", i.Number, i.Title, i.Author)
		}

		*issues = nil
	}

	return r
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
