# canary のリリース手順を doc/DEV.md に追加する

- Created: 2026-11-25
- Completed: {YYYY-MM-DD}
- Branch: feature/update-canary-release-procedure
- Polished: {YYYY-MM-DD}

## 目的

canary のタグ付けと VERSION の更新の手順を、今の運用に合わせてドキュメントに残す。

## 現状

- `doc/DEV.md` にはリリースビルドの手順はあるが、canary のタグ付けと VERSION の更新の手順がない。
- 直近の `2026.2.4-canary.0` は、hotfix ブランチで VERSION を 2026.2.4 に上げ、`2026.2.4-canary.0` のタグを付けて push し、その後 main にマージして `2026.2.4` のタグを付ける運用になっている。
- `canary.py` を削除する前に、canary の版の上げ方をドキュメントに残す必要がある。

## 設計方針

- `doc/DEV.md` に canary のリリース手順を追加する。
- 今の運用に合わせて、VERSION の更新、CHANGES.md の更新、コミット、`<VERSION>-canary.0` のタグ付け、push、main へのマージ、`<VERSION>` のタグ付けの流れを書く。

## 完了条件

- `doc/DEV.md` に canary のリリース手順がある。
