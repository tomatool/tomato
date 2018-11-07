package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tomatool/tomato/dictionary"
	"github.com/tomatool/tomato/generate/docs"
	"github.com/tomatool/tomato/generate/handler"
	"github.com/tomatool/tomato/version"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	dictionaryPath string
	outputPath     string
	outputType     string
)

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "tomatool - tomato tools")
	app.Version(version.Print())
	app.HelpFlag.Short('h')

	generateCmd := app.Command("generate", "generate")
	generateDocsCmd := generateCmd.Command("docs", "generate documentation")
	generateDocsCmd.Flag("dictionary", "tomato dictionary file path.").Short('d').Default("dictionary.yml").StringVar(&dictionaryPath)
	generateDocsCmd.Flag("output", "output of handler.go.").Short('o').Default("docs/resources.md").StringVar(&outputPath)
	generateDocsCmd.Flag("type", "output type (markdown/html).").Short('t').Default("html").StringVar(&outputType)

	generateHandlerCmd := generateCmd.Command("handler", "generate handler")
	generateHandlerCmd.Flag("dictionary", "tomato dictionary file path.").Short('d').Default("dictionary.yml").StringVar(&dictionaryPath)
	generateHandlerCmd.Flag("output", "output of handler.go.").Short('o').Default("handler/handler.go").StringVar(&outputPath)

	var err error
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case generateHandlerCmd.FullCommand():
		err = GenerateHandler(dictionaryPath, outputPath)
	case generateDocsCmd.FullCommand():
		err = GenerateDocs(dictionaryPath, outputPath, outputType)
	}
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func GenerateDocs(dictionaryPath, outputPath, outputType string) error {
	dict, err := dictionary.Retrieve(dictionaryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error retrieving config"))
		os.Exit(1)
	}

	out, err := docs.Generate(dict, &docs.Options{Output: outputType})
	if err != nil {
		return err
	}

	return ioutil.WriteFile(outputPath, out.Bytes(), 0755)
}

func GenerateHandler(dictionaryPath, outputPath string) error {
	dict, err := dictionary.Retrieve(dictionaryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error retrieving config"))
		os.Exit(1)
	}

	out, err := handler.Generate(dict)
	if err != nil {
		return err
	}

	for resourceName, r := range out {
		if err := ioutil.WriteFile("./handler/"+resourceName+"/handler.go", r.Bytes(), 0755); err != nil {
			return err
		}
	}

	return nil
}
