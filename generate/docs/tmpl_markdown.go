package docs

import "io/ioutil"

var (
	markdownTmpl string
)

func init() {
	f, err := ioutil.ReadFile("./generate/docs/tmpl.md")
	if err != nil {
		panic(err)
	}
	markdownTmpl = string(f)
}
