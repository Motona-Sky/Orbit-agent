#!/usr/bin/env python3
import argparse
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request


def parse_args():
    parser = argparse.ArgumentParser(description="测试 ChatGPT Codex 模型列表接口")
    parser.add_argument(
        "--access-token",
        default=os.getenv("CODEX_ACCESS_TOKEN", ""),
        help="Codex access token，默认读取 CODEX_ACCESS_TOKEN",
    )
    parser.add_argument(
        "--account-id",
        default=os.getenv("CHATGPT_ACCOUNT_ID", ""),
        help="ChatGPT account ID，默认读取 CHATGPT_ACCOUNT_ID",
    )
    parser.add_argument(
        "--base-url",
        default="https://chatgpt.com/backend-api",
        help="Codex API 基础地址",
    )
    parser.add_argument("--client-version", default="0.144.1")
    parser.add_argument("--timeout", type=float, default=15)
    return parser.parse_args()


def redact(value):
    if len(value) <= 12:
        return "***"
    return value[:6] + "..." + value[-6:]


def main():
    args = parse_args()
    if not args.access_token.strip():
        print("缺少 access token：请设置 CODEX_ACCESS_TOKEN 或传入 --access-token", file=sys.stderr)
        return 2

    url = args.base_url.rstrip("/") + "/codex/models"
    url += "?" + urllib.parse.urlencode({"client_version": args.client_version})
    headers = {
        "Authorization": "Bearer " + args.access_token.strip(),
        "Accept": "application/json",
        "Originator": "codex_cli_rs",
        "User-Agent": "codex_cli_rs/" + args.client_version,
    }
    if args.account_id.strip():
        headers["ChatGPT-Account-Id"] = args.account_id.strip()

    print("请求地址:", url)
    print("请求头:")
    for key, value in headers.items():
        if key == "Authorization":
            value = "Bearer " + redact(args.access_token.strip())
        print(f"  {key}: {value}")

    request = urllib.request.Request(url, headers=headers, method="GET")
    try:
        with urllib.request.urlopen(request, timeout=args.timeout) as response:
            body = response.read().decode("utf-8", errors="replace")
            print("HTTP 状态:", response.status)
            print("Content-Type:", response.headers.get("Content-Type", ""))
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace")
        print("HTTP 状态:", error.code, error.reason, file=sys.stderr)
        print("响应头:", file=sys.stderr)
        for key, value in error.headers.items():
            print(f"  {key}: {value}", file=sys.stderr)
        print("响应体:", file=sys.stderr)
        print(body, file=sys.stderr)
        return 1
    except urllib.error.URLError as error:
        print("请求失败:", error.reason, file=sys.stderr)
        return 1

    try:
        payload = json.loads(body)
        print("响应 JSON:")
        print(json.dumps(payload, ensure_ascii=False, indent=2))
    except json.JSONDecodeError:
        print("响应不是 JSON:")
        print(body)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
