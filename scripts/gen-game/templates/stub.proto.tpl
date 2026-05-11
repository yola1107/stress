syntax = "proto3";

// 与 api/common/pb/stub.proto 同形；改语义时请两边（及 stub.pb.go.tpl / marshal）同步。
// 嵌入式 FileDescriptorProto.name 故意与 api 一致：common/pb/stub.proto（非磁盘路径）。

option go_package = "{{ .GoPackagePB }}";

package {{ .ProtoPkg }};

// {{ .Name }}（GameID: {{ .ID }}）
message Stub_BetOrderResponse {
}
