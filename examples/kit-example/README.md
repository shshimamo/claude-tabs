# sbx kit サンプル

sbx の [kit](https://docs.docker.com/ai/sandboxes/customize/kits/) を使って sandbox にツール・環境変数・ネットワーク設定等を追加できる。

## 使い方

### 1. config.json に kit を追加

```json
{
  "sbx_kits": [
    "~/path/to/kit-example/",
    "ghcr.io/myorg/my-kit:1.0"
  ]
}
```

ローカルディレクトリ、ZIP、OCI イメージを指定可能。複数指定で積み重ねられる。

### 2. kit の構成

```
kit-example/
├── spec.yaml          # kit 定義（必須）
├── README.md
└── files/             # sandbox にコピーするファイル（任意）
    └── workspace/
        └── .tool-config
```

`files/` 配下のファイルは sandbox 内の対応するパスにコピーされる。

### 3. kit の検証・配布

```sh
sbx kit validate ./kit-example/          # 検証
sbx kit pack ./kit-example/ -o kit.zip   # ZIP 化
sbx kit push ./kit-example/ ghcr.io/org/kit:1.0  # レジストリに公開
```

## spec.yaml の主要フィールド

| フィールド | 説明 |
|-----------|------|
| `kind` | `mixin`（既存エージェント拡張）または `sandbox`（エージェント定義） |
| `commands.install` | 作成時に実行。イメージにキャッシュされる |
| `commands.startup` | 起動時に毎回実行。`background: true` でデーモン化可 |
| `environment.variables` | 環境変数の設定 |
| `network.allowedDomains` | 通信を許可するドメイン |
| `network.deniedDomains` | 通信を拒否するドメイン |
