# 使われていない canary.py と copilot-instructions.md を削除する

- Created: 2026-11-20
- Completed: {YYYY-MM-DD}
- Branch: feature/remove-unused-files
- Polished: {YYYY-MM-DD}

## 目的

今の運用で使われていないファイルと設定を削除し、リポジトリの内容と実際の運用を一致させる。

## 現状

- `canary.py` は、VERSION を canary の版に上げて、コミット、タグ付け、push を行うスクリプトである。
  - このスクリプトが作るコミット (`[canary] バージョンを ... にあげる`) は、`git log --all` に 1 件もない。
  - 最近の canary のタグ (`2026.2.4-canary.0` など) は、VERSION がリリースの版のままで、FIX の位を上げた canary である。このスクリプトは RELEASE の位を上げた canary か、canary の番号を上げたものしか作れない。
  - パス指定なしの `git commit` のため、ステージ済みの無関係な変更も含めてコミットし、そのまま push する。
  - Python の実行環境 (pyproject.toml や PEP 723 のメタデータ) の指定がなく、shiguredo-python の規約 (uv / ruff / ty) にも沿っていない。
- `copilot-instructions.md` は、`- 日本語で反応して` の 1 行だけのファイルである。GitHub Copilot が読むのは `.github/copilot-instructions.md` のため、リポジトリの直下に置いても読まれない。内容も AGENTS.md と重なっている。
- `.gitignore` の `.grafana/*` に対応するものが、リポジトリにも手順にもない。

## 設計方針

- `canary.py`、`copilot-instructions.md`、`.gitignore` の `.grafana/*` を削除する。
- canary のリリースの手順が別に必要な場合は、今の運用に合わせて別の issue で扱う。

## 完了条件

- 上記のファイルと設定がリポジトリにない。
