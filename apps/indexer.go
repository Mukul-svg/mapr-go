// Inverted-index MapReduce application.
// Map emits (word, filename) for each unique word in a document.
// Reduce outputs the list of documents containing each word.
package apps

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	mr "mapreduce-go/mr"
)

func IndexerMap(document string, value string) []mr.KeyValue {
	seen := make(map[string]bool)
	words := strings.FieldsFunc(value, func(r rune) bool { return !unicode.IsLetter(r) })

	res := []mr.KeyValue{}
	for _, w := range words {
		if !seen[w] {
			seen[w] = true
			res = append(res, mr.KeyValue{Key: w, Value: document})
		}
	}
	return res
}

func IndexerReduce(key string, values []string) string {
	sort.Strings(values)
	return fmt.Sprintf("%d %s", len(values), strings.Join(values, ","))
}
