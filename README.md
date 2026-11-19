# meguru（巡る）

自律エージェント／プロジェクトの「改善ループ」を管理する軽量CLIツールです。
静的バイナリ1つで動き、テレメトリはなく、データはすべてファイルベースです。

## michaku との役割分担

| ツール | 扱うもの | 問い |
|---|---|---|
| michaku | 運用ログ・異常検出 | 何が起きたか |
| **meguru** | ループの設計と改善 | 何を良くするか |

meguru は実行を制御しません。ループの定義（目的・指標・段階の設計・改善の履歴）を
検証し、要約し、陳腐化や実績との乖離を検出するだけです。観測そのものは michaku 側の領分です。

## 定義フォーマット（YAML）

```yaml
loop: catalog-refresh             # 必須: ループ名
purpose: 公開カタログを回すたびに良くする  # 必須
updated_at: "2026-11-15"          # 任意 (YYYY-MM-DD)
current_stage: 収集               # 任意
cycles: 3                         # 任意: 周回数

metrics:                          # 指標
  - name: 公開件数                 # 必須
    current: "120"
    target: "200"                 # validate 必須
    measured_by: data/catalog.csv # validate 必須
    updated: "2026-11-15"         # 任意（無ければ loop の updated_at を使う）

stages:                           # 段階（実行設計）
  - name: 収集                     # 必須
    split: 項目単位                # validate 必須: 分割単位
    parallel_max: 4               # parallel_max か parallel_note のどちらかが必須
    # parallel_note: 資源の性質で振り分ける
    reason: I/O待ちが大半のため    # validate 必須

improvements:                     # 改善台帳
  - date: "2026-11-15"
    change: 写真の出典ルールを整理した
    aimed_at: 公開件数             # 定義済み指標名を指すこと（validate で検査）
    result: "110件 → 120件"        # 空 = 未記録（check で検出）

next_candidates:                  # 次の改善候補
  - 誤り報告フォームの実装
```

日付フィールドは文字列として扱い、`2006-01-02` 形式と RFC3339 の両方に対応します。

## コマンド

### validate — 定義の検証

```sh
meguru validate <file|dir>...
```

検証項目: ループ名がある／purpose がある／全指標に target と measured_by がある／
全段階に split と reason があり parallel_max か parallel_note のどちらかがある／
improvements の aimed_at が定義済み指標名を指している／YAML として読める。
問題があればファイル名と項目を列挙して終了コード 1、なければ `OK` で終了コード 0。

### report — ループ一覧のダイジェスト

```sh
meguru report [--format md|json] <file|dir>...
```

ループ名・目的・現在段階・周回数、各指標の現在値→目標、直近の改善（最大3件）、
次の改善候補を表示します。デフォルトは Markdown、`--format json` で機械可読な構造を出力します。

### check — 陳腐化・未記録・乖離の検出

```sh
meguru check [--actuals <jsonl>] [--stale-days N] [--now <date>] <file|dir>...
```

検出項目:

1. **指標の陳腐化**: `metrics[].updated`（無ければループの `updated_at`）から
   `--now`（デフォルト今日）までが `--stale-days`（デフォルト30）を超えたら finding
2. **結果未記録の改善**: improvements の `result` が空なら finding
3. **実績との乖離**（`--actuals` 指定時のみ）: 段階ごとの最大同時実行数が
   設計 `parallel_max` を超えたら finding（`parallel_max` が無い段階は比較しない）

finding があれば列挙して終了コード 1、なければ `no findings` で終了コード 0。

## 実績JSONL形式

1行1JSON。フィールド名の揺れを吸収します（読み取り専用・緩い対応）:

| 意味 | 受け付ける名前 |
|---|---|
| 段階名 | `stage` / `stage_name` / `name` |
| 開始 | `started_at` / `start` / `begin` |
| 終了 | `finished_at` / `end` / `finish` |
| 成否 | `status`（ok/success/succeeded/failed/failure/error 等）/ `success`（bool）/ `ok`（bool） |

```json
{"stage":"収集","started_at":"2026-11-15T09:00:00+09:00","finished_at":"2026-11-15T09:12:00+09:00","status":"ok"}
```

段階ごとに実行回数・失敗率・平均所要時間・最大同時実行数（区間の重なりから算出）を集計します。
読めない行は警告してスキップし、全体は落としません。

## ビルド

```sh
go build -o meguru .
go test ./...
```

サンプル定義は `examples/` にあります（施設カタログの更新ループ／検証研究ループ／実績サンプル）。

## ライセンス

MIT — [LICENSE](LICENSE) を参照。
