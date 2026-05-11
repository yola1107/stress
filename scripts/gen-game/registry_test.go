package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestPatchRegistryAddsImportAndEntry(t *testing.T) {
	const before = `package game

import (
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g1"
)

var registry = map[int64]base.IGame{
	g1.ID: g1.New(), // a
}
`
	after, err := patchRegistry(before, "g999991", "新游戏测")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(after, `"stress/internal/biz/game/g999991"`) {
		t.Fatalf("missing import:\n%s", after)
	}
	if !strings.Contains(after, "g999991.ID: g999991.New(), // 新游戏测") {
		t.Fatalf("missing map entry:\n%s", after)
	}
	if !(strings.Contains(after, "// a") && strings.Contains(after, "g1.New()")) {
		t.Fatalf("应保留 g1 行及尾随注释:\n%s", after)
	}
	if strings.Contains(after, "\n\n\tg999991") {
		t.Fatalf("新条目前应无空行:\n%s", after)
	}
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "", after, parser.ParseComments); err != nil {
		t.Fatalf("parsed Go invalid: %v\n%s", err, after)
	}
}

func TestPatchRegistryDuplicateMapEntry(t *testing.T) {
	const before = `package game

import (
	"stress/internal/biz/game/base"
	"stress/internal/biz/game/g1"
)

var registry = map[int64]base.IGame{
	g1.ID: g1.New(), // x
}
`
	_, err := patchRegistry(before, "g1", " dup")
	if err == nil || !strings.Contains(err.Error(), "已有") {
		t.Fatalf("expect duplicate error, got: %v", err)
	}
}

func TestPatchRegistryRejectsNewlineName(t *testing.T) {
	const snippet = `package game

import (
	"stress/internal/biz/game/base"
)

var registry = map[int64]base.IGame{}
`
	_, err := patchRegistry(snippet, "g9", "a\nb")
	if err == nil {
		t.Fatal("expect error for newline in name")
	}
}

func TestMarshalStubDescriptorRoundTrip(t *testing.T) {
	d := newGameSpec(77777, "测")
	bs, err := marshalStubFileDescriptorWire(d)
	if err != nil {
		t.Fatal(err)
	}
	var fd descriptorpb.FileDescriptorProto
	if err := proto.Unmarshal(bs, &fd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, want := fd.GetName(), "common/pb/stub.proto"; got != want {
		t.Fatalf("descriptor name: %q want %q", got, want)
	}
	if got, want := fd.GetSyntax(), "proto3"; got != want {
		t.Fatalf("syntax: %q want %q", got, want)
	}
	if fd.GetPackage() != d.ProtoPkg {
		t.Fatalf("package")
	}
	if len(fd.MessageType) != 1 || fd.MessageType[0].GetName() != "Stub_BetOrderResponse" {
		t.Fatalf("message")
	}
}
