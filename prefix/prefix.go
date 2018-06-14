package prefix

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gernest/gs/browserlist"
	"github.com/gernest/gs/ciu/agents"
	"github.com/gernest/gs/data"
)

var re = regexp.MustCompile(`^(-\w+-)`)

func Prefix(s string) string {
	sub := re.FindSubmatch([]byte(s))
	if sub != nil {
		return string(sub[0])
	}
	return ""
}

func UnPrefix(s string) string {
	return string(re.ReplaceAll([]byte(s), []byte("")))
}

type Browser struct {
	Prefixcache  []string
	PrefixRegexp *regexp.Regexp
	Selected     []string
	Data         map[string]agents.Agent
}

func NewBrowser(data map[string]agents.Agent, queries ...string) (*Browser, error) {
	selected, err := browserlist.Query(queries...)
	if err != nil {
		return nil, err
	}
	re := ""
	for _, a := range agents.New() {
		pre := "-" + a.Prefix + "-"
		if re != "" {
			re += "|" + pre
		} else {
			re = pre
		}
	}
	reg := regexp.MustCompile(re)
	return &Browser{Selected: selected, Data: data, PrefixRegexp: reg}, nil
}

func uniq(s []string) []string {
	m := make(map[string]bool)
	for _, v := range s {
		m[v] = true
	}
	var o []string
	for k := range m {
		o = append(o, k)
	}
	return o
}

func splitBrowser(b string) (string, float64) {
	parts := strings.Split(b, " ")
	k := parts[0]
	if len(parts) == 1 || parts[1] == "" {
		return k, 0
	}
	v, _ := strconv.ParseFloat(parts[1], 64)
	return k, v
}

func sortBrowsers(v []string) {
	sort.SliceStable(v, func(i, j int) bool {
		a, av := splitBrowser(v[i])
		b, bv := splitBrowser(v[j])
		return a > b || math.Signbit(av-bv)
	})
}

func (b *Browser) WithPrefix(value string) bool {
	return b.PrefixRegexp.Match([]byte(value))
}

func (b *Browser) Prefix(name string) string {
	p := strings.Split(name, " ")
	name, version := p[0], p[1]
	d := b.Data[name]
	prefix := ""
	if d.PrefixExceptions != nil {
		prefix = d.PrefixExceptions[version]
	}
	if prefix == "" {
		prefix = d.Prefix
	}
	return "-" + prefix + "-"
}

func (b *Browser) Prefixes() []string {
	if b.Prefixcache != nil {
		return b.Prefixcache
	}
	for _, v := range agents.New() {
		b.Prefixcache = append(b.Prefixcache, "-"+v.Prefix+"-")
	}
	b.Prefixcache = uniq(b.Prefixcache)
	return b.Prefixcache
}

func (b *Browser) IsSelected(name string) bool {
	if b.Selected != nil {
		return sliceContains(b.Selected, name)
	}
	return false
}

type SelectedOptions struct {
	Add    map[string][]string
	Remove map[string][]string
}

type Prefixes struct {
	Browsers *Browser
	Data     map[string]data.Data
	Opts     *PrefixesOptions
}

type PrefixesOptions struct {
	FlexBox string
}

type addOpts struct {
	browser string
	note    string
}

func (p *Prefixes) Select(list map[string]data.Data) *SelectedOptions {
	selected := &SelectedOptions{
		Add:    make(map[string][]string),
		Remove: make(map[string][]string),
	}
	for name := range list {
		data := list[name]
		var add []addOpts
		for _, v := range data.Browsers {
			parts := strings.Split(v, " ")
			o := addOpts{
				browser: fmt.Sprintf("%s %s", parts[0], parts[1]),
			}
			if len(parts) == 3 {
				o.note = parts[2]
			}
			add = append(add, o)
		}
		var notes []string
		for _, v := range add {
			if v.note != "" {
				notes = append(notes,
					fmt.Sprintf("%s %s", p.Browsers.Prefix(v.browser), v.note),
				)
			}
		}
		notes = uniq(notes)
		var addList []string
		fadd := filterAddOptions(add, func(v addOpts) bool {
			sl := p.Browsers.IsSelected(v.browser)
			return sl
		})
		for _, v := range fadd {
			prefx := p.Browsers.Prefix(v.browser)
			if v.note != "" {
				addList = append(addList,
					fmt.Sprintf("%s %s", prefx, v.note),
				)
			} else {
				addList = append(addList, prefx)
			}
		}
		addList = uniq(addList)
		sort.Strings(addList)
		if p.Opts != nil && p.Opts.FlexBox == "no-2009" {
			addList = filter(addList, func(v string) bool {
				return !strings.Contains(v, "2009")
			})
		}
		all := mapSlice(data.Browsers, func(v string) string {
			return p.Browsers.Prefix(v)
		})
		if data.Mistakes != nil {
			all = append(all, data.Mistakes...)
		}
		if notes != nil {
			all = append(all, notes...)
		}
		all = uniq(all)
		if len(addList) > 0 {
			selected.Add[name] = addList
			if len(addList) < len(all) {
				rm := filter(all, func(v string) bool {
					return !sliceContains(addList, v)
				})
				selected.Remove[name] = rm
			}
		} else {
			selected.Remove[name] = all
		}
	}
	return selected
}

func sliceContains(s []string, v string) bool {
	for k := range s {
		if s[k] == v {
			return true
		}
	}
	return false
}

func filter(src []string, fn func(string) bool) []string {
	var ns []string
	for _, v := range src {
		if fn(v) {
			ns = append(ns, v)
		}
	}
	return ns
}

func filterAddOptions(src []addOpts, fn func(addOpts) bool) []addOpts {
	var ns []addOpts
	for _, v := range src {
		if fn(v) {
			ns = append(ns, v)
		}
	}
	return ns
}

func mapSlice(m []string, fn func(string) string) []string {
	var ns []string
	for _, v := range m {
		ns = append(ns, fn(v))
	}
	return ns
}
