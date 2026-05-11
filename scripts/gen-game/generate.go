package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

//go:embed templates/game.tpl
var rawGameTpl string

//go:embed templates/stub.proto.tpl
var rawStubProtoTpl string

//go:embed templates/stub.pb.go.tpl
var rawStubPbGoTpl string

var (
	tplGame      *template.Template
	tplStubProto *template.Template
	tplStubPbGo  *template.Template
)

func init() {
	parse := func(name, raw string) *template.Template {
		return template.Must(template.New(name).Parse(strings.TrimSpace(raw)))
	}
	tplGame = parse("game.tpl", rawGameTpl)
	tplStubProto = parse("stub.proto.tpl", rawStubProtoTpl)
	tplStubPbGo = parse("stub.pb.go.tpl", rawStubPbGoTpl)
}

// gameSpec 与 templates/*.tpl 中 {{ .字段 }} 一致。
type gameSpec struct {
	Pkg          string
	ID           int64
	Name         string
	NameGoQuoted string
	GoPackagePB  string
	ProtoPkg     string
}

func newGameSpec(id int64, displayName string) *gameSpec {
	pkg := fmt.Sprintf("g%d", id)
	return &gameSpec{
		Pkg:          pkg,
		ID:           id,
		Name:         displayName,
		NameGoQuoted: strconv.Quote(displayName),
		GoPackagePB:  filepath.ToSlash(filepath.Join("stress", "internal", "biz", "game", pkg, "pb")),
		ProtoPkg:     fmt.Sprintf("stub_g%d", id),
	}
}

func loadSpecFromEnv() (*gameSpec, error) {
	idStr := strings.TrimSpace(os.Getenv("GAME_ID"))
	name := strings.TrimSpace(os.Getenv("GAME_NAME"))
	if idStr == "" || name == "" {
		return nil, fmt.Errorf("用法: GAME_ID=<id> GAME_NAME=<名称> go run ./scripts/gen-game")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("GAME_ID 须为正整数")
	}
	return newGameSpec(id, name), nil
}

func findGoModuleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for d := wd; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		if d == filepath.Dir(d) {
			break
		}
	}
	return "", fmt.Errorf("未找到 go.mod")
}

type stubPbTemplateData struct {
	*gameSpec
	RawDescConst string
}

func renderStubProto(s *gameSpec) ([]byte, error) {
	return executeTemplate(tplStubProto, s)
}

func renderGameGo(s *gameSpec) ([]byte, error) {
	return executeTemplate(tplGame, s)
}

func renderStubPbGo(s *gameSpec) ([]byte, error) {
	wire, err := marshalStubFileDescriptorWire(s)
	if err != nil {
		return nil, err
	}
	rawDesc := "const file_common_pb_stub_proto_rawDesc = " + strconv.Quote(string(wire)) + "\n"
	var buf bytes.Buffer
	if err := tplStubPbGo.Execute(&buf, stubPbTemplateData{s, rawDesc}); err != nil {
		return nil, err
	}
	return format.Source(buf.Bytes())
}

func executeTemplate(t *template.Template, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("模板 %s: %w", t.Name(), err)
	}
	return buf.Bytes(), nil
}

func marshalStubFileDescriptorWire(s *gameSpec) ([]byte, error) {
	fd := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("common/pb/stub.proto"),
		Package: proto.String(s.ProtoPkg),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String(s.GoPackagePB),
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Stub_BetOrderResponse")},
		},
	}
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(fd)
	if err != nil {
		return nil, fmt.Errorf("marshal FileDescriptorProto: %w", err)
	}
	return b, nil
}

func writeGameScaffold(moduleRoot string, s *gameSpec) error {
	gameDir := filepath.Join(moduleRoot, "internal", "biz", "game", s.Pkg)
	protoDir := filepath.Join(gameDir, "pb")
	if st, err := os.Stat(gameDir); err == nil && st.IsDir() {
		return fmt.Errorf("目录已存在: %s", gameDir)
	}
	if err := os.MkdirAll(protoDir, 0o755); err != nil {
		return fmt.Errorf("创建目录: %w", err)
	}

	for _, step := range []struct {
		label string
		path  string
		gen   func(*gameSpec) ([]byte, error)
		gofmt bool
	}{
		{"stub.proto", filepath.Join(protoDir, "stub.proto"), renderStubProto, false},
		{"stub.pb.go", filepath.Join(protoDir, "stub.pb.go"), renderStubPbGo, false},
		{"game.go", filepath.Join(gameDir, "game.go"), renderGameGo, true},
	} {
		b, err := step.gen(s)
		if err != nil {
			return fmt.Errorf("%s: %w", step.label, err)
		}
		if step.gofmt {
			b, err = format.Source(b)
			if err != nil {
				return fmt.Errorf("gofmt %s: %w", step.label, err)
			}
		}
		if err := os.WriteFile(step.path, b, 0o644); err != nil {
			return fmt.Errorf("写入 %s: %w", step.label, err)
		}
	}
	return nil
}

func appendGameToRegistry(moduleRoot string, s *gameSpec) error {
	p := filepath.Join(moduleRoot, "internal", "biz", "game", "registry.go")
	raw, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("读取 registry.go: %w", err)
	}
	if regexp.MustCompile(fmt.Sprintf(`\bg%d\b`, s.ID)).Match(raw) {
		return fmt.Errorf("registry.go 已包含包名 g%d（请手动检查后再试）", s.ID)
	}
	next, err := patchRegistry(string(raw), s.Pkg, s.Name)
	if err != nil {
		return fmt.Errorf("改写 registry.go: %w", err)
	}
	if err := os.WriteFile(p, []byte(next), 0o644); err != nil {
		return fmt.Errorf("写入 registry.go: %w", err)
	}
	return nil
}
