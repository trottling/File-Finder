package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkLoadPatterns(b *testing.B) {
	dir := b.TempDir()
	fp := filepath.Join(dir, "p.txt")
	var body string
	for i := 0; i < 2000; i++ {
		body += "plain:i:hello\n"
	}
	body += "re:^user=\\w+$\n"
	_ = os.WriteFile(fp, []byte(body), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := LoadPatterns(fp)
		if err != nil {
			b.Fatal(err)
		}
	}
}
