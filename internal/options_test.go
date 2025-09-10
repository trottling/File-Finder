package internal

import "testing"

func TestScanOptions_Validate(t *testing.T) {
	o := ScanOptions{}
	if err := o.Validate(); err == nil {
		t.Fatal("expected error when pattern-file empty")
	}
	o.PatternFile = "p.txt"
	o.SaveFull = true
	o.SaveFullFolder = ""
	if err := o.Validate(); err == nil {
		t.Fatal("expected error when --save-full without folder")
	}
	o.SaveFullFolder = "/x"
	if err := o.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestScanOptions_PrepareAndAllowedExt(t *testing.T) {
	o := ScanOptions{
		PatternFile: "p.txt",
		Whitelist:   []string{".txt", ".log"},
		Blacklist:   []string{".bin"},
	}
	o.Prepare()
	if !o.allowedExt(".txt") || !o.allowedExt(".log") {
		t.Fatal("whitelist must allow listed ext")
	}
	if o.allowedExt(".bin") {
		t.Fatal("whitelist must ignore blacklist entirely")
	}

	// no whitelist - blacklist only
	o = ScanOptions{PatternFile: "p.txt", Blacklist: []string{".tmp", ".bin"}}
	o.Prepare()
	if o.allowedExt(".tmp") || o.allowedExt(".bin") {
		t.Fatal("blacklist must block ext")
	}
	if !o.allowedExt(".txt") {
		t.Fatal("non-blacklisted ext must pass")
	}
}
