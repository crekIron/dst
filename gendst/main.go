package main

import (
	"sort"

	"github.com/dave/dst/gendst/fragment"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	var names []string
	for name := range fragment.Info {
		names = append(names, name)
	}
	sort.Strings(names)

	if err := generateDst(names); err != nil {
		return err
	}
	if err := generateDstDecs(names); err != nil {
		return err
	}
	if err := generateFragger(names); err != nil {
		return err
	}
	if err := generateDecorator(names); err != nil {
		return err
	}
	if err := generateDecoratorTestHelper(names); err != nil {
		return err
	}
	if err := generateRestorer(names); err != nil {
		return err
	}
	return nil
}
