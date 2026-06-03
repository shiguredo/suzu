import argparse
import subprocess
from typing import Optional


# ファイルを読み込み、バージョンを更新
def update_version(file_path: str, dry_run: bool) -> Optional[str]:
    with open(file_path, "r", encoding="utf-8") as f:
        current_version: str = f.read().strip()

    # バージョンが -canary.X を持っている場合の更新
    if "-canary." in current_version:
        # canary バージョンをインクリメント
        base_version, canary_suffix = current_version.rsplit("-canary.", 1)
        new_version = f"{base_version}-canary.{int(canary_suffix) + 1}"
    else:
        # -canary.X がない場合、次のリリースバージョンにして -canary.0 を追加
        parts = current_version.split(".")
        if len(parts) != 3:
            raise ValueError("Version format in VERSION file is not YEAR.RELEASE.FIX")
        year, release, fix = map(int, parts)
        new_version = f"{year}.{release + 1}.0-canary.0"

    print(f"Current version: {current_version}")
    print(f"New version: {new_version}")
    confirmation: str = input("Do you want to update the version? (y/N): ").strip().lower()

    if confirmation != "y":
        print("Version update canceled.")
        return None

    # Dry-run 時の動作
    if dry_run:
        print("Dry-run: Version would be updated to:")
        print(new_version)
    else:
        with open(file_path, "w", encoding="utf-8") as f:
            f.write(f"{new_version}\n")
        print(f"Version updated in {file_path} to {new_version}")

    return new_version


# git コミット、タグ、プッシュを実行
def git_operations(new_version: str, dry_run: bool) -> None:
    if dry_run:
        print("Dry-run: Would run 'git add VERSION'")
        print(f"Dry-run: Would run 'git commit -m [canary] バージョンを {new_version} にあげる'")
        print(f"Dry-run: Would run 'git tag {new_version}'")
        print("Dry-run: Would run 'git push'")
        print("Dry-run: Would run 'git push --tags'")
    else:
        subprocess.run(["git", "add", "VERSION"], check=True)
        subprocess.run(
            ["git", "commit", "-m", f"[canary] バージョンを {new_version} にあげる"], check=True
        )
        subprocess.run(["git", "tag", new_version], check=True)
        subprocess.run(["git", "push"], check=True)
        subprocess.run(["git", "push", "--tags"], check=True)


# メイン処理
def main() -> None:
    parser = argparse.ArgumentParser(
        description="Update VERSION file and commit changes."
    )
    parser.add_argument(
        "--dry-run", action="store_true", help="Run in dry-run mode without making actual changes"
    )
    args = parser.parse_args()

    version_file_path: str = "VERSION"

    # バージョン更新
    new_version: Optional[str] = update_version(version_file_path, args.dry_run)

    if not new_version:
        return  # ユーザーが確認をキャンセルした場合、処理を中断

    # git 操作
    git_operations(new_version, args.dry_run)


if __name__ == "__main__":
    main()
