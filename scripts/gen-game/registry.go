package main

import (
	"fmt"
	"go/ast"
	goformat "go/format"
	"go/parser"
	"go/token"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const stressGameBaseImport = `stress/internal/biz/game/base`

// registryParsed 保存一次 ParseFile 的结果与 import/registry 结点，供后续按字节拼接。
type registryParsed struct {
	src  string
	fset *token.FileSet
	imp  *ast.GenDecl
	cl   *ast.CompositeLit
}

func parseRegistryFile(src string) (*registryParsed, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "registry.go", src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("解析 registry.go: %w", err)
	}
	imp, err := findGroupedImportDecl(file)
	if err != nil {
		return nil, err
	}
	cl, err := findRegistryMapLiteral(file)
	if err != nil {
		return nil, err
	}
	return &registryParsed{src: src, fset: fset, imp: imp, cl: cl}, nil
}

func (r *registryParsed) hasRegistryEntryForPkg(pkg string) bool {
	for _, elt := range r.cl.Elts {
		if registryKeyIsPkg(elt, pkg) {
			return true
		}
	}
	return false
}

// patchRegistry 在 registry.go 源码中追加游戏 import 与 map 一行，并 gofmt。
// 策略：AST 定位 + 源级切片替换 import 括号内文本、在 map 的「}」前插入新行（避免 printer 与行尾 // 注释错位）。
func patchRegistry(src, pkg, displayName string) (string, error) {
	if strings.ContainsAny(displayName, "\r\n") {
		return "", fmt.Errorf("GAME_NAME 不能含换行")
	}

	parsed, err := parseRegistryFile(src)
	if err != nil {
		return "", err
	}
	if parsed.hasRegistryEntryForPkg(pkg) {
		return "", fmt.Errorf("registry 已有 %s.ID 条目", pkg)
	}

	paths, err := collectImportPaths(parsed.imp)
	if err != nil {
		return "", err
	}
	want := fmt.Sprintf("stress/internal/biz/game/%s", pkg)
	if !slices.Contains(paths, want) {
		paths = append(paths, want)
	}

	lp := parsed.fset.Position(parsed.imp.Lparen).Offset
	rp := parsed.fset.Position(parsed.imp.Rparen).Offset
	r0 := parsed.fset.Position(parsed.cl.Rbrace).Offset
	if lp < 0 || rp < 0 || r0 < 0 || lp >= len(parsed.src) || rp > len(parsed.src) || r0 > len(parsed.src) || lp >= rp {
		return "", fmt.Errorf("registry.go 源码 offset 无效")
	}

	oldInner := parsed.src[lp+1 : rp]
	newInner := formatImportBlockInner(paths)
	delta := len(newInner) - len(oldInner)
	r1 := r0 + delta

	out := parsed.src[:lp+1] + newInner + parsed.src[rp:]
	if r1 < 0 || r1 > len(out) {
		return "", fmt.Errorf("registry「}」offset 修正后越界")
	}

	prefix := "\n"
	if r1 > 0 && out[r1-1] == '\n' {
		prefix = ""
	}
	line := fmt.Sprintf("%s\t%s.ID: %s.New(), // %s\n", prefix, pkg, pkg, displayName)
	out = out[:r1] + line + out[r1:]

	formatted, err := goformat.Source([]byte(out))
	if err != nil {
		return "", fmt.Errorf("gofmt registry.go: %w", err)
	}
	return string(formatted), nil
}

func findGroupedImportDecl(file *ast.File) (*ast.GenDecl, error) {
	for _, d := range file.Decls {
		g, ok := d.(*ast.GenDecl)
		if ok && g.Tok == token.IMPORT && g.Lparen.IsValid() {
			return g, nil
		}
	}
	return nil, fmt.Errorf("未找到分组 import (...)")
}

func findRegistryMapLiteral(file *ast.File) (*ast.CompositeLit, error) {
	for _, d := range file.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.VAR || len(g.Specs) != 1 {
			continue
		}
		vs, ok := g.Specs[0].(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "registry" || len(vs.Values) != 1 {
			continue
		}
		cl, ok := vs.Values[0].(*ast.CompositeLit)
		if !ok || cl.Rbrace == token.NoPos {
			return nil, fmt.Errorf("registry 须为 map literal")
		}
		if _, ok := cl.Type.(*ast.MapType); !ok {
			return nil, fmt.Errorf("registry 须为 map 类型 literal")
		}
		return cl, nil
	}
	return nil, fmt.Errorf("未找到 var registry")
}

func registryKeyIsPkg(elt ast.Expr, pkg string) bool {
	kv, ok := elt.(*ast.KeyValueExpr)
	if !ok {
		return false
	}
	sel, ok := kv.Key.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != "ID" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

func collectImportPaths(imp *ast.GenDecl) ([]string, error) {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range imp.Specs {
		is, ok := s.(*ast.ImportSpec)
		if !ok {
			continue
		}
		if is.Name != nil {
			return nil, fmt.Errorf("import 含别名（暂不支持改写）")
		}
		p, err := strconv.Unquote(is.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("解析 import 路径: %w", err)
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out, nil
}

func formatImportBlockInner(paths []string) string {
	seen := make(map[string]struct{})
	var uniq []string
	for _, p := range paths {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		uniq = append(uniq, p)
	}
	var base, rest []string
	for _, p := range uniq {
		if p == stressGameBaseImport {
			base = append(base, p)
			continue
		}
		rest = append(rest, p)
	}
	sort.Strings(rest)
	ordered := append(append([]string{}, base...), rest...)

	var b strings.Builder
	for _, p := range ordered {
		b.WriteString("\n\t")
		b.WriteString(strconv.Quote(p))
	}
	b.WriteByte('\n')
	return b.String()
}
