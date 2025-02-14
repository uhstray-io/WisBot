package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rotisserie/eris"
)

func PrintTrace(err error) {
	// format := eris.NewDefaultStringFormat(eris.FormatOptions{
	// 	InvertOutput: true, // flag that inverts the error output (wrap errors shown first)
	// 	WithTrace:    true, // flag that enables stack trace output
	// 	InvertTrace:  true, // flag that inverts the stack trace output (top of call stack shown first)
	// 	WithExternal: true,
	// })
	// fmt.Println(eris.ToCustomString(err, format))

	upErr := eris.Unpack(err)

	var str string
	if upErr.ErrExternal != nil {
		str += fmt.Sprintf("%+v", upErr.ErrExternal) + "\n"
	}
	str += fmt.Sprintf("%+v", upErr.ErrRoot.Msg) + "\n"

	for _, frame := range upErr.ErrRoot.Stack {
		str += frame.Name + "\n"
		str += "\t" + removeParentFolder(frame.File) + ":" + strconv.Itoa(frame.Line) + "\n"
	}

	str += "\n"

	for _, eLink := range upErr.ErrChain {
		str += eLink.Msg + "\n"
		str += eLink.Frame.Name + "\n"
		str += "\t" + removeParentFolder(eLink.Frame.File) + ":" + strconv.Itoa(eLink.Frame.Line) + "\n\n"
	}

	if err != nil {
		fmt.Print(str)
	}
}

func ErrorTrace(err error) {
	// format := eris.NewDefaultStringFormat(eris.FormatOptions{
	// 	InvertOutput: true, // flag that inverts the error output (wrap errors shown first)
	// 	WithTrace:    true, // flag that enables stack trace output
	// 	InvertTrace:  true, // flag that inverts the stack trace output (top of call stack shown first)
	// 	WithExternal: true,
	// })
	// fmt.Println(eris.ToCustomString(err, format))

	upErr := eris.Unpack(err)

	var str string
	if upErr.ErrExternal != nil {
		str += fmt.Sprintf("%+v", upErr.ErrExternal) + "\n"
	}
	str += fmt.Sprintf("%+v", upErr.ErrRoot.Msg) + "\n"

	for _, frame := range upErr.ErrRoot.Stack {
		str += frame.Name + "\n"
		str += "\t" + removeParentFolder(frame.File) + ":" + strconv.Itoa(frame.Line) + "\n"
	}

	str += "\n"

	for _, eLink := range upErr.ErrChain {
		str += eLink.Msg + "\n"
		str += eLink.Frame.Name + "\n"
		str += "\t" + removeParentFolder(eLink.Frame.File) + ":" + strconv.Itoa(eLink.Frame.Line) + "\n\n"
	}

	if err != nil {
		fmt.Print(str)
		os.Exit(1)
	}
}

// Function that removes the parent path from the file path
func removeParentFolder(parentfolder string) string {
	SplitLabel := "Wisbot" // Change this to the parent directory name
	SplitPath := strings.Split(parentfolder, SplitLabel)

	if len(SplitPath) == 1 {
		return parentfolder
	}

	return SplitPath[1]
}
