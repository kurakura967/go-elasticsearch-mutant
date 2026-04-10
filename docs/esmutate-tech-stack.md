# go-elasticsearch-mutant - 技術選定ドキュメント

## 概要

Elasticsearch Query DSL に対するミューテーションテスティング CLI ツール。
ユーザーの Go テストスイートが、ES クエリの変更を正しく検出できるかを評価する。

| 項目 | 値 |
|------|-----|
| リポジトリ名 | `go-elasticsearch-mutant` |
| CLI コマンド名 | `esmutant` |
| インストール | `go install github.com/<user>/go-elasticsearch-mutant/cmd/esmutant@latest` |

---

## 前提条件

- 言語: Go
- 対象: go-elasticsearch v8 の **Typed API** で組み立てられた検索クエリ
- ES クラスタ: ユーザー側で起動済み
- 実行形態: CLI ツール
- 出力: 標準出力に mutation score を表示

---

## アーキテクチャ

### アプローチ: Go AST 書き換え方式

指定パッケージの Go ソースコードを `go/ast` で静的解析し、
go-elasticsearch Typed API の構造体フィールドを直接書き換えてテストを再実行する。

```
esmutant run ./search/...

1. 指定パッケージの .go ファイルを go/ast で解析
2. esapi Typed API の呼び出し箇所を特定
   (例: types.BoolQuery の Must フィールドへの代入)
3. ミュータントを生成
   (例: BoolQuery.Must → BoolQuery.Should にフィールド書き換え)
4. 書き換えたファイルを一時ファイルとして書き出す
5. overlay.json を生成（元ファイルパス → 一時ファイルのマッピング）
6. go test -overlay overlay.json ./search/... を元ディレクトリで実行
7. fail → killed / pass → survived
8. 結果を集計して標準出力に表示
```

### 採用理由

- パッケージ指定により go/ast での静的解析が可能
- Typed API は ES クエリ構造が Go の型として表現されており、AST 上で機械的に検出・書き換えできる
- gomu (github.com/sivchari/gomu) と同じパラダイムであり、実績のあるアプローチ
- HTTP プロキシが不要でアーキテクチャがシンプル
- レポートにファイル名・行番号を含められるため、ユーザーにとって actionable

### 不採用とした方式

| 方式 | 不採用理由 |
|------|-----------|
| クエリ JSON 入力方式 | ユーザーが設定ファイルを手書きする必要があり、既存テストスイートの品質評価にならない |
| HTTP プロキシ方式 | クエリ箇所の特定が困難。プロキシ実装の複雑さに対して得られる価値が見合わない |

### ミュータント実行: `go test -overlay` 方式

Go 1.16 で導入された `-overlay` フラグを使い、ファイルの仮想的な差し替えによるテスト実行を行う。
overlay は JSON 設定ファイルで「ディスク上のパス → 別のファイルの内容」というマッピングを定義し、
ビルドシステムがあたかもそのファイルが差し替えられたかのように動作する仕組み。

```json
{
  "Replace": {
    "/home/user/project/search/query.go": "/tmp/esmutant_work/mutant_001.go"
  }
}
```

```bash
go test -overlay /tmp/esmutant_work/overlay_001.json ./search/...
```

#### overlay 方式の利点（一時ディレクトリコピー方式との比較）

**1. ビルドキャッシュが効く**

最大の利点。overlay は Go のビルドキャッシュと正しく連携するため、
変更していないパッケージのコンパイル結果はキャッシュから再利用される。
ミューテーションテスティングでは1ファイルだけが変わるケースがほとんどなので、
2回目以降のミュータントのテスト実行が大幅に高速化される。

**2. ファイルシステム操作が最小限**

プロジェクト全体のコピーが不要。変更した1ファイル（数KB）+ overlay.json（数十バイト）だけ書けばよい。
大規模プロジェクトほど差が顕著になる。

**3. 元のソースが汚れない**

元ファイルを一切触らないため、テスト中にプロセスが異常終了してもソースが破損するリスクがない。
go-mutesting 等の従来ツールは元のファイルをミュータントで直接置き換える方式のため、
クラッシュ時にソースが壊れる問題があった。

**4. 並列実行との相性が良い**

ワーカーごとに独立した overlay.json と一時ファイルを用意するだけで、
ファイルシステムレベルでの競合が一切発生しない。
一時ディレクトリ方式ではプロジェクトコピーの排他制御が必要になる場合がある。

#### overlay 実行フローの詳細

```
# ミュータント1つあたりの処理
1. rewriter が AST を書き換えて go/format でソース出力
2. /tmp/esmutant_work/mutant_<id>.go として書き出す
3. overlay.json を生成:
   {"Replace": {"/abs/path/to/search/query.go": "/tmp/esmutant_work/mutant_<id>.go"}}
4. 元プロジェクトディレクトリで実行:
   go test -overlay /tmp/esmutant_work/overlay_<id>.json -count=1 ./search/...
5. 終了コードで killed/survived を判定
6. 一時ファイル削除（数KB なので高速）
```

---

## CLI インターフェース

```bash
# 基本
$ esmutant run ./search/...

# オプション付き
$ esmutant run ./search/... --workers 4 --threshold 80.0 --timeout 30
```

### コマンド

| コマンド | 説明 |
|---------|------|
| `esmutant run [package]` | 指定パッケージに対してミューテーションテストを実行 |
| `esmutant version` | バージョン情報を表示 |

### フラグ

| フラグ | デフォルト | 説明 |
|-------|-----------|------|
| `--workers` | `4` | 並列ワーカー数 |
| `--threshold` | `80.0` | 最低 mutation score (0-100) |
| `--timeout` | `30` | テストタイムアウト秒 |
| `--output` | `console` | 出力形式 (console / json) |
| `--verbose` | `false` | 詳細出力 |

### 出力イメージ

```
$ esmutant run ./search/...

Analyzing package: ./search/...
Found 3 esapi call sites in 2 test files

Generating mutants...
  search_test.go:42  BoolQuery.Must → Should
  search_test.go:42  BoolQuery.Must[0] (term) → removed
  search_test.go:58  RangeQuery.Gte → Gt
  search_test.go:71  BoolQuery.MustNot → removed
  ... (12 mutants total)

Running mutations [████████░░] 8/12

Mutation Score: 9/12 (75.0%)

SURVIVED:
  search_test.go:42  BoolQuery.Must → Should
  search_test.go:58  RangeQuery.Gte → Gt
  search_test.go:71  BoolQuery.MustNot → removed
```

---

## 技術スタック

| 領域 | 技術 | 備考 |
|------|------|------|
| 言語 | Go 1.21+ | |
| CLI フレームワーク | `github.com/spf13/cobra` | gomu と同じ |
| 静的解析 | `go/ast` + `go/parser` + `go/types` | 標準ライブラリ |
| AST 書き換え | `go/ast` + `go/format` | 標準ライブラリ |
| ミュータント実行 | `go test -overlay` | Go 1.16+ のビルドフラグ。ファイル仮想差し替え |
| テスト実行 | `os/exec` | `go test` を子プロセスで実行 |
| overlay 生成 | `encoding/json` | overlay.json の生成 |
| 出力フォーマット | 標準出力 (テーブル形式 / JSON) | |
| 外部依存 | cobra のみ | 最小限に抑える |

### 主な標準ライブラリの役割

| パッケージ | 用途 |
|-----------|------|
| `go/parser` | 指定パッケージの .go ファイルをパースして AST を取得 |
| `go/ast` | AST を走査して Typed API 呼び出し箇所を検出、フィールドを書き換え |
| `go/types` | 型情報を使って `types.BoolQuery` 等の構造体を正確に識別 |
| `go/format` | 書き換えた AST を Go ソースコードに再出力 |
| `encoding/json` | overlay.json の生成 |
| `os/exec` | `go test -overlay` を子プロセスとして実行 |

---

## ミューテーションオペレータ (第1フェーズ: ドキュメント集合変更系)

| # | オペレータ | 書き換え内容 | 検出するテストの弱点 |
|---|----------|-------------|-------------------|
| 1 | `RemoveClause` | bool 内の must/filter/must_not の句を1つ削除 | 条件が欠落しても気づけない |
| 2 | `MustToShould` | `BoolQuery.Must` → `BoolQuery.Should` | AND/OR の違いを検証していない |
| 3 | `RangeBoundary` | `Gte` → `Gt`, `Lte` → `Lt` | 境界値のテストが不足 |
| 4 | `RemoveMustNot` | `BoolQuery.MustNot` を空にする | 除外条件をテストしていない |
| 5 | `SwapField` | term/match のフィールド名を同型の別フィールドに置換 | 正しいフィールドを参照しているか未検証 |

### 第2フェーズ (将来): スコアリング変更系

- `boost` 値の変更
- `function_score` の関数削除
- `match` → `match_phrase` 置換

---

## ディレクトリ構成

```
go-elasticsearch-mutant/
├── cmd/
│   └── esmutant/
│       └── main.go           # エントリポイント
├── example/
│   ├── docker-compose.yml   # ES 8.x シングルノード起動用
│   ├── testdata/
│   │   └── seed.json        # テスト用ドキュメント投入データ
│   ├── search.go            # Typed API を使った検索関数群
│   └── search_test.go       # 検索結果を検証するテスト
├── internal/
│   ├── analyzer/
│   │   ├── analyzer.go      # パッケージ解析: esapi 呼び出し箇所の検出
│   │   └── callsite.go      # 検出した呼び出し箇所の構造体定義
│   ├── mutant/
│   │   ├── operator.go      # ミューテーションオペレータのインターフェースと実装
│   │   ├── generator.go     # 呼び出し箇所 × オペレータからミュータント一覧を生成
│   │   └── rewriter.go      # AST 書き換え処理
│   ├── runner/
│   │   ├── executor.go      # go test 子プロセス実行
│   │   ├── overlay.go       # overlay.json 生成・一時ファイル管理
│   │   ├── worker.go        # 並列実行のワーカープール
│   │   └── result.go        # pass/fail 判定
│   └── report/
│       ├── console.go       # 標準出力レポート
│       └── json.go          # JSON レポート
├── go.mod
├── go.sum
└── README.md
```

### 各コンポーネントの責務

```
[CLI (cobra)]
    │
    ▼
[analyzer] 指定パッケージを go/ast で解析
    │       → esapi Typed API の呼び出し箇所一覧を返す
    ▼
[mutant/generator] 呼び出し箇所 × オペレータ でミュータント一覧を生成
    │
    ▼
[mutant/rewriter] 各ミュータントに対して AST を書き換え
    │               → go/format でソースに変換
    ▼
[runner/overlay] 書き換えたソースを一時ファイルに書き出し
    │             → overlay.json を生成
    ▼
[runner/executor] go test -overlay overlay.json を元ディレクトリで実行
    │              → killed / survived を判定
    ▼
[report] 結果を集計して標準出力に表示
```

---

## 開発の進め方: example ディレクトリによる動作確認

### 目的

esmutant の実装を進める前に、**ミューテーション対象となる「典型的な検索コード」** を
`example/` ディレクトリに用意する。これにより以下が可能になる。

- Typed API で構築したクエリの AST 構造を実際に確認できる
- esmutant のオペレータが正しく動作するかの E2E 的な検証対象になる
- ミューテーションが入った場合にテストが落ちる（= killed になる）ことを手動で確かめられる

### example/ のディレクトリ構成

```
example/
├── docker-compose.yml          # ES 8.x シングルノード起動用
├── testdata/
│   └── seed.json               # テスト用ドキュメントの投入データ
├── search.go                   # Typed API を使った検索関数群
└── search_test.go              # 検索結果を検証するテスト
```

### search.go に実装する検索パターン

esmutant の初期オペレータ5種をすべてカバーする検索関数を用意する。

| 関数 | クエリ構造 | 対応オペレータ |
|------|----------|--------------|
| `SearchActiveUsersByName` | bool.must[match(name)] + bool.filter[term(status:"active")] | RemoveClause, MustToShould |
| `SearchProductsInPriceRange` | bool.must[match(category)] + bool.filter[range(price: gte/lte)] | RangeBoundary |
| `SearchArticlesExcluding` | bool.must[match(title)] + bool.must_not[term(status:"draft")] | RemoveMustNot |
| `SearchByField` | bool.must[term(email:...)] | SwapField |

### search_test.go の方針

- ES への接続は `ELASTICSEARCH_URL` 環境変数（デフォルト: `http://localhost:9200`）
- `TestMain` でテストデータ投入・インデックス作成を行い、テスト終了後にクリーンアップ
- 各テストは「返却されるドキュメントの件数」と「特定ドキュメントの有無」で assertion する
- テストデータは少数（10件程度）で、各ドキュメントのフィールド値を明確に設計し、
  クエリ条件の変更が結果に確実に影響するようにする

### テストデータ設計方針

ミューテーションが入ったときにテストが **確実に fail する** ことが重要。
そのためにテストデータは以下を満たすよう設計する。

- `must` → `should` に変えたら本来ヒットしないドキュメントが混入する
- `gte` → `gt` に変えたら境界値ちょうどのドキュメントが落ちる
- `must_not` を外したら除外対象のドキュメントが結果に含まれる
- フィールド名を変えたら全く別のデータでマッチする

### 実装の進め方

1. **まず example/ を完成させる** — ES を起動し、検索とテストが通ることを確認
2. **手動でミューテーションを試す** — 例えば `Must` を `Should` に手で書き換えてテストが落ちることを確認
3. **analyzer の実装** — example/search.go を解析対象にして、Typed API 呼び出し箇所が検出できるか確認
4. **rewriter の実装** — example/search.go の AST を実際に書き換えて、正しいミュータントが生成されるか確認
5. **runner の実装** — overlay で example/ のテストを実行し、killed/survived が正しく判定されるか確認

example/ は実装完了後もリポジトリに残し、CI でも回す。
esmutant 自体の E2E テストとしての役割も兼ねる。

---

## 参考プロジェクト

| プロジェクト | 参考にする点 |
|-------------|-------------|
| [gomu](https://github.com/sivchari/gomu) | Go AST ベースのミューテーションテスティングの全体設計、CLI 設計、並列実行、レポーティング |
| [Mutant Swarm](https://github.com/HiveRunner/mutant-swarm) | クエリに対するミューテーションテスティングの概念設計、カバレッジレポートの構造 |
| [TdRules/SQLMutation](https://github.com/giis-uniovi/tdrules) | SQL ミューテーションオペレータの分類体系 (ES 向けオペレータ設計の参考) |
| [go-mutesting](https://github.com/zimmski/go-mutesting) | 先行する Go ミューテーションツール。元ファイル直接置換方式の問題点（クラッシュ時のソース破損）が overlay 採用の動機 |
| [gremlins](https://github.com/go-gremlins/gremlins) | Go ミューテーションツールの CI 統合パターン、failfast 戦略の参考 |
