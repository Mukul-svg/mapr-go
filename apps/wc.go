// Word-count MapReduce application.
// Map emits (word, "1") for each word in the input.
// Reduce counts the occurrences of each word.
package apps

import (
	"strings"
	"strconv"
	"unicode"

	mr "mapreduce-go/mr"
)

func WcMap(filename string, contents string) []mr.KeyValue {
	// Split on any non-letter character.
	ff := func(r rune) bool { return !unicode.IsLetter(r) }
	words := strings.FieldsFunc(contents, ff)

	kva := []mr.KeyValue{}
	for _, w := range words {
		kva = append(kva, mr.KeyValue{Key: w, Value: "1"})
	}
	return kva
}

func WcReduce(key string, values []string) string {
	return strconv.Itoa(len(values))
}
