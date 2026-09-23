#!/usr/bin/env python3
"""将华为云 SWR 镜像仓库设置为公开/私有。

使用华为云标准 SDK-HMAC-SHA256 签名直接调用 UpdateRepo 接口：
    PATCH /v2/manage/namespaces/{namespace}/repos/{repository}
    body: {"is_public": true, "description": "public"}
    成功返回 201

用法：
    export HUAWEI_AK=xxx HUAWEI_SK=xxx
    python3 swr_set_public.py --region cn-north-4 --namespace my-org \
        --repository 'docker.io/rancher/mirrored-pause'

repository 中的 '/' 会自动转换为 '$'（SWR 文档要求）。
AK/SK 也可以用 --ak/--sk 参数传入，建议用环境变量。
"""
import argparse
import hashlib
import hmac
import os
import sys
import urllib.parse
import urllib.request
from datetime import datetime, timezone

SUCCESS_CODE = 201


def percent_encode(s: str) -> str:
    """RFC3986 编码，保留非保留字符。"""
    return urllib.parse.quote(s, safe="-_.~")


def canonical_uri(path: str) -> str:
    """CanonicalURI：逐段编码后拼回，并按规范在末尾补 '/'。"""
    segs = path.strip("/").split("/")
    return "/" + "/".join(percent_encode(s) for s in segs) + "/"


def sign(method: str, path: str, body: str, ak: str, sk: str, host: str):
    """按华为云签名规范生成 Authorization 头。"""
    sdk_date = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    c_uri = canonical_uri(path)
    signed = "content-type;host;x-sdk-date"
    canon_headers = (
        f"content-type:application/json\n"
        f"host:{host}\n"
        f"x-sdk-date:{sdk_date}\n"
    )
    body_hash = hashlib.sha256(body.encode()).hexdigest()
    canonical_request = f"{method}\n{c_uri}\n\n{canon_headers}\n{signed}\n{body_hash}"
    string_to_sign = (
        "SDK-HMAC-SHA256\n"
        f"{sdk_date}\n"
        f"{hashlib.sha256(canonical_request.encode()).hexdigest()}"
    )
    signature = hmac.new(
        sk.encode(), string_to_sign.encode(), hashlib.sha256
    ).hexdigest()
    authorization = (
        f"SDK-HMAC-SHA256 Access={ak}, SignedHeaders={signed}, Signature={signature}"
    )
    debug = f"--- CanonicalRequest ---\n{canonical_request}\n--- StringToSign ---\n{string_to_sign}"
    return sdk_date, authorization, debug


def main():
    parser = argparse.ArgumentParser(description="Set SWR repository public")
    parser.add_argument("--region", required=True, help="如 cn-north-4")
    parser.add_argument("--namespace", required=True, help="SWR 组织名")
    parser.add_argument("--repository", required=True, help="仓库名，'/' 会转成 '$'")
    parser.add_argument("--private", action="store_true", help="设置为私有（默认公开）")
    parser.add_argument("--ak", default=os.environ.get("HUAWEI_AK"))
    parser.add_argument("--sk", default=os.environ.get("HUAWEI_SK"))
    parser.add_argument("--debug", action="store_true", help="打印签名过程")
    args = parser.parse_args()

    if not args.ak or not args.sk:
        sys.exit("错误：请通过环境变量 HUAWEI_AK/HUAWEI_SK 或 --ak/--sk 提供 AK/SK")

    repo = args.repository.replace("/", "$")
    host = f"swr-api.{args.region}.myhuaweicloud.com"
    path = f"/v2/manage/namespaces/{args.namespace}/repos/{repo}"
    body = '{"is_public": %s, "description": "public"}' % (
        "false" if args.private else "true"
    )

    sdk_date, authorization, debug = sign(
        "PATCH", path, body, args.ak, args.sk, host
    )
    if args.debug:
        print(debug, file=sys.stderr)

    req = urllib.request.Request(
        f"https://{host}{path}",
        data=body.encode(),
        method="PATCH",
        headers={
            "Content-Type": "application/json",
            "X-Sdk-Date": sdk_date,
            "Authorization": authorization,
        },
    )
    try:
        with urllib.request.urlopen(req) as resp:
            status = resp.status
            resp_body = resp.read().decode()
    except urllib.error.HTTPError as e:
        status = e.code
        resp_body = e.read().decode()

    if status == SUCCESS_CODE:
        print(f"✅ {args.namespace}/{args.repository} 已设置为{'私有' if args.private else '公开'}")
    else:
        print(f"❌ 失败，HTTP {status}，响应：{resp_body}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
