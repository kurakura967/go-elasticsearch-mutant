# go-elasticsearch-mutant 実装方針

## 実装フェーズ概要

技術スタックドキュメントで定義した「example ファーストの進め方」に従い、以下の順序で実装する。

| フェーズ | 内容 | 完了条件 |
|---------|------|---------|
| 1 | `example/` の整備 | ES を起動してテストが通る |
| 2 | `internal/analyzer` の実装 | example/search.go の Typed API 呼び出し箇所を検出できる |
| 3 | `internal/mutant` の実装 | ミュータントを生成・AST 書き換えができる |
| 4 | `internal/runner` の実装 | overlay で example/ のテストを実行し killed/survived が判定できる |
| 5 | CLI 統合 + report の実装 | `esmutant run ./example/...` がエンドツーエンドで動く |

---

## フェーズ1: example/ の整備

### docker-compose.yml

ES 8.x シングルノードをセキュリティ無効で起動する。

```yaml
services:
  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.13.0
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false
    ports:
      - "9200:9200"
```

### testdata/seed.json

各ミューテーションオペレータが **確実に killed になる** ようにドキュメントを設計する。

```
ドキュメント設計方針:
- must → should に変えたら、本来ヒットしないドキュメントが結果に混入する
- gte → gt に変えたら、境界値ちょうどのドキュメントが落ちる
- must_not を外したら、除外対象ドキュメントが結果に含まれる
- フィールド名を変えたら、全く別のデータにマッチする
```

投入するインデックスと件数の目安:

| インデックス | 件数 | 用途 |
|------------|------|------|
| `users` | 8件 | RemoveClause, MustToShould, SwapField |
| `products` | 6件 | RangeBoundary |
| `articles` | 6件 | RemoveMustNot |

### search.go に実装する関数

クエリの **実行** と **構築** を分離する設計にする。

```go
// ES クライアントのラッパー
type esClient struct {
    client *elasticsearch.TypedClient
}

// 共通の検索実行メソッド。クエリを受け取って ES に投げるだけ
// search.Response は github.com/elastic/go-elasticsearch/v8/typedapi/core/search
func (c *esClient) Search(ctx context.Context, index string, query *types.Query) (*search.Response, error)

// RemoveClause, MustToShould 対応
// bool.must[match(name)] + bool.filter[term(status:"active")]
func BuildActiveUsersQuery(name string) *types.Query

// RangeBoundary 対応
// bool.must[match(category)] + bool.filter[range(price: gte/lte)]
func BuildPriceRangeQuery(category string, minPrice, maxPrice float64) *types.Query

// RemoveMustNot 対応
// bool.must[match(title)] + bool.must_not[term(status:"draft")]
func BuildArticlesQuery(title string) *types.Query

// SwapField 対応
// bool.must[term(email:...)]
func BuildUserByEmailQuery(email string) *types.Query
```

ミューテーションオペレータの書き換え対象は `Build*` 関数内の Typed API 呼び出し箇所になる。
`Search` メソッドはミュータントの対象にならない。

### search_test.go の方針

- ES 接続先: `ELASTICSEARCH_URL` 環境変数（デフォルト: `http://localhost:9200`）
- `TestMain` でインデックス作成 → seed データ投入 → テスト実行 → インデックス削除
- アサーション: 期待するドキュメント一覧と実際の検索結果が完全に一致することを検証する
- `testing.Short()` のときはスキップ（CI でオプショナルにできるよう）

---

## フェーズ2: internal/analyzer の実装

### 役割

指定パッケージの `.go` ファイルを `go/ast` + `go/types` で解析し、
go-elasticsearch Typed API の呼び出し箇所（`CallSite`）一覧を返す。

### CallSite 構造体

```go
// internal/analyzer/callsite.go
type CallSite struct {
    File     string       // 絶対パス
    Line     int          // 行番号
    NodeType string       // "BoolQuery", "RangeQuery" など
    Field    string       // "Must", "Gte" など
    Node     ast.Node     // 書き換え対象の AST ノード（rewriter が使う）
}
```

### 検出ロジック

`go/types` の型情報を使い、構造体リテラルのフィールド代入を正確に識別する。

```
1. go/packages でパッケージをロード（TypesInfo 付き）
2. ast.Inspect で *ast.KeyValueExpr を走査
3. TypesInfo.TypeOf(kv.Key) が github.com/elastic/go-elasticsearch/v8/typedapi/types パッケージの
   既知の構造体フィールドであれば CallSite として記録
4. 対象フィールドのホワイトリスト（Must, Should, Filter, MustNot, Gte, Gt, Lte, Lt など）で絞り込み
```

### analyzer.go のインターフェース

```go
// internal/analyzer/analyzer.go
type Analyzer struct {
    Dir string // パッケージのディレクトリ
}

func (a *Analyzer) Analyze(pattern string) ([]*CallSite, error)
```

---

## フェーズ3: internal/mutant の実装

### Operator インターフェース

```go
// internal/mutant/operator.go
type Operator interface {
    Name() string
    Apply(site *analyzer.CallSite) ([]*Mutant, error)
}
```

### Mutant 構造体

```go
// internal/mutant/generator.go
type Mutant struct {
    ID          int
    Site        *analyzer.CallSite
    Operator    string       // オペレータ名
    Description string       // "BoolQuery.Must → Should" など
    ModifiedSrc []byte       // go/format で出力した書き換え後ソース
}
```

### 各オペレータの実装方針

#### 1. RemoveClause

```
対象: BoolQuery の Must / Filter / Should / MustNot フィールドへのスライス代入
書き換え: 代入値を nil または空スライスに置換
```

#### 2. MustToShould

```
対象: BoolQuery.Must への代入
書き換え: フィールド名 "Must" → "Should" に変更（KeyValueExpr.Key を書き換え）
```

#### 3. RangeBoundary

```
対象: NumberRangeQuery / DateRangeQuery の Gte / Lte フィールド
書き換え:
  Gte → Gt
  Lte → Lt
各フィールドごとに独立したミュータントを生成
```

#### 4. RemoveMustNot

```
対象: BoolQuery.MustNot への代入
書き換え: 代入値を nil に置換（RemoveClause の特化版）
```

#### 5. SwapField

```
対象: TermQuery / MatchQuery のフィールド名引数
書き換え: 同一インデックス内の別フィールド名（同型）に置換
※ フィールド名の候補は同ファイル内の他 TermQuery / MatchQuery から収集
```

### rewriter.go の実装

```
1. 元ファイルを go/parser でパース
2. CallSite の Node を特定（ファイル・行番号で照合）
3. ast.Node を書き換え（フィールド名変更 or 値の nil 置換）
4. go/format.Node で []byte に変換して返す
```

---

## フェーズ4: internal/runner の実装

### overlay.go

```go
type OverlayManager struct {
    WorkDir string // /tmp/esmutant_work
}

// ミュータントのソースを一時ファイルに書き出し overlay.json を生成
func (m *OverlayManager) Write(mutant *mutant.Mutant) (overlayPath string, cleanup func(), err error)
```

overlay.json の形式:

```json
{
  "Replace": {
    "/abs/path/to/original.go": "/tmp/esmutant_work/mutant_<id>.go"
  }
}
```

### executor.go

```go
type Executor struct {
    ProjectDir string
    Timeout    time.Duration
}

// go test -overlay <overlayPath> -count=1 <pattern> を実行
func (e *Executor) Run(overlayPath, pattern string) (Result, error)
```

### result.go

```go
type Status int

const (
    Killed   Status = iota // テストが fail → ミュータントを検出できた
    Survived               // テストが pass → ミュータントを検出できなかった
    Timeout                // タイムアウト
    Error                  // コンパイルエラー等
)

type Result struct {
    MutantID int
    Status   Status
    Output   string // go test の標準出力
}
```

### worker.go — 並列実行

```
- ワーカー数は --workers フラグで指定（デフォルト 4）
- ミュータント一覧をチャネルで各ワーカーに配布
- 各ワーカーは独立した overlay.json と一時ファイルを使うため競合なし
- context でタイムアウト制御
```

---

## フェーズ5: CLI 統合 + report の実装

### cmd/esmutant/main.go

cobra を使ったエントリポイント。

```
root
├── run [package]   ← メインコマンド
└── version
```

### run コマンドの処理フロー

```go
func runCommand(pattern string, opts Options) error {
    // 1. Analyze
    sites, _ := analyzer.New(dir).Analyze(pattern)

    // 2. Generate mutants
    mutants, _ := mutant.Generate(sites, operators)

    // 3. Run
    results, _ := runner.RunAll(mutants, pattern, opts)

    // 4. Report
    return report.Print(results, opts.Output)
}
```

### report/console.go

```
出力イメージ（tech-stack.md の出力例に準拠）:
- プログレスバー
- Mutation Score: X/Y (Z%)
- SURVIVED セクション（ファイル名:行番号 + 書き換え内容）
- threshold 未達の場合は exit code 1
```

### report/json.go

```json
{
  "score": 75.0,
  "total": 12,
  "killed": 9,
  "survived": 3,
  "mutants": [...]
}
```

---

## テスト戦略

| コンポーネント | テスト方針 |
|--------------|---------|
| `analyzer` | `example/search.go` をフィクスチャとして使い、検出される CallSite の内容を検証 |
| `mutant/operator` | 小さな Go ソースを文字列で用意し、書き換え後のソースをスナップショット比較 |
| `mutant/rewriter` | 同上。`go/format` 後の文字列で比較 |
| `runner/overlay` | 実際に一時ファイルを書き出し、overlay.json の内容を検証 |
| `runner/executor` | `go test -overlay` の実行は example/ を使った統合テストで検証 |
| E2E | `example/` を対象に `esmutant run` を実行し、Mutation Score が期待値以上であることを確認 |

---

## CI 設計

```yaml
jobs:
  unit-test:
    # ES 不要。analyzer / mutant の単体テストのみ
    run: go test ./internal/...

  e2e-test:
    services:
      elasticsearch: # docker-compose と同等の設定
    steps:
      - go test -v ./example/...           # example/ のテスト単体が通ること
      - go run ./cmd/esmutant run ./example/... --threshold 80
```

example/ は実装完了後もリポジトリに残し、esmutant 自身の E2E テストとして機能させる。
