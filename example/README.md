# example

`esmutant` の動作確認・E2E テスト用パッケージ。

## 役割

- ミューテーション対象となる「典型的な検索コード」のサンプル実装
- `esmutant` の各ミューテーションオペレータが正しく機能するかの E2E 的な検証対象
- `esmutant run ./example/...` を実行したとき、ミューテーションが入った場合にテストが fail することを確認できる

## 構成

```
example/
├── docker-compose.yml          # Elasticsearch 8.x シングルノード起動用
├── search.go                   # esClient と Build* クエリ構築関数
├── search_test.go              # 検索結果を検証するテスト
└── testdata/
    └── fixtures/
        ├── users/              # users インデックスのフィクスチャ
        │   ├── _mapping.json
        │   └── documents.yml
        ├── products/           # products インデックスのフィクスチャ
        │   ├── _mapping.json
        │   └── documents.yml
        └── articles/           # articles インデックスのフィクスチャ
            ├── _mapping.json
            └── documents.yml
```

## テストの実行

### 1. Elasticsearch を起動する

```bash
cd example
docker compose up -d
```

### 2. テストを実行する

```bash
go test ./example/...
```

デフォルトでは `http://localhost:9200` に接続します。接続先を変更する場合は `ELASTICSEARCH_URL` 環境変数を設定してください。

```bash
ELASTICSEARCH_URL=http://localhost:9200 go test ./example/...
```

### 3. Elasticsearch を停止する

```bash
cd example
docker compose down
```

## ミューテーションオペレータとテストの対応

| テスト | クエリ構造 | 検出するオペレータ |
|--------|-----------|------------------|
| `TestBuildActiveUsersQuery` | `bool.must[match(name)]` + `bool.filter[term(status)]` | `RemoveClause`, `MustToShould` |
| `TestBuildPriceRangeQuery` | `bool.must[match(category)]` + `bool.filter[range(price: gte/lte)]` | `RangeBoundary` |
| `TestBuildArticlesQuery` | `bool.must[match(title)]` + `bool.must_not[term(status)]` | `RemoveMustNot` |
| `TestBuildUserByEmailQuery` | `bool.must[term(email)]` | `SwapField` |
