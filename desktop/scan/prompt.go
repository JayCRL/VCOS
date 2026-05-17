package scan

// SemanticPrompt is the fixed instruction we send to `claude --print` for
// the L3 layer. We force a pure-JSON response so the front-end can parse it
// straight into the Semantic struct.
const SemanticPrompt = `请用 Read / Glob / Grep 真实地扫描当前项目目录(README 优先),
然后产出**严格的 JSON**,字段如下。不要任何解释、不要 markdown 包裹、不要前后任何字符,直接以 ` + "`{`" + ` 开始 ` + "`}`" + ` 结束:

{
  "summary": "一句话项目摘要(15-30 字)",
  "language": "主语言,如 go | typescript | python | rust | mixed",
  "entryPoints": [
    {"path": "相对路径", "purpose": "入口用途一句话"}
  ],
  "modules": [
    {"name": "模块名", "path": "顶级目录或文件", "responsibility": "职责一句话"}
  ],
  "deps": [
    {"from": "模块A 名", "to": "模块B 名", "kind": "imports | calls | depends"}
  ],
  "hotspots": [
    {"path": "文件路径", "reason": "为什么是热点:复杂、最近改动多、关键风险等"}
  ]
}

约束:
- modules 不超过 8 个,只列顶层
- entryPoints 不超过 5 个
- deps 只列模块之间的关键依赖,不下钻到叶子文件
- hotspots 选 3-5 个最值得关注的
- 路径用项目相对路径(不含开头的 ./)
- 输出必须是单一 JSON 对象,首尾无任何额外字符或 markdown 围栏`
