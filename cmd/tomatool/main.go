package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/alileza/tomato/dictionary"
	"github.com/alileza/tomato/generate/docs"
	"github.com/alileza/tomato/generate/handler"
	"github.com/alileza/tomato/util/version"
	"github.com/pkg/errors"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	dictionaryPath string
	outputPath     string
)

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "tomatool - tomato tools")
	app.Version(version.Print())
	app.HelpFlag.Short('h')

	generateCmd := app.Command("generate", "generate")
	generateDocsCmd := generateCmd.Command("docs", "generate documentation")
	generateDocsCmd.Flag("dictionary", "tomato dictionary file path.").Short('d').StringVar(&dictionaryPath)
	generateDocsCmd.Flag("output", "output of handler.go.").Short('o').Default("docs/resources.md").StringVar(&outputPath)

	generateHandlerCmd := generateCmd.Command("handler", "generate handler")
	generateHandlerCmd.Flag("dictionary", "tomato dictionary file path.").Short('d').StringVar(&dictionaryPath)
	generateHandlerCmd.Flag("output", "output of handler.go.").Short('o').Default("handler/handler.go").StringVar(&outputPath)

	var err error
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case generateHandlerCmd.FullCommand():
		err = GenerateHandler(dictionaryPath, outputPath)
	case generateDocsCmd.FullCommand():
		err = GenerateDocs(dictionaryPath, outputPath)
	}
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func GenerateDocs(dictionaryPath, outputPath string) error {
	dict, err := dictionary.Retrieve(dictionaryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Error retrieving config"))
		os.Exit(1)
	}

	out, err := docs.Generate(dict, nil)
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

	return ioutil.WriteFile(outputPath, out.Bytes(), 0755)
}
