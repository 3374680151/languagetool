#!/usr/bin/env python3
# gen-dict.py 从 ECDICT 全量 CSV 生成精简词典（DR-6：约 3000 高频词随二进制 embed）。
#
# @file scripts/gen-dict.py
# 文件职责：读 ECDICT csv（word,phonetic,definition,translation,pos,collins,oxford,tag,bnc,frq,exchange），
# 按词频（frq）取前 N 个高频英文词条，输出紧凑 JSON 到 internal/dict/data/ecdict-slim.json。
# 只保留查词卡需要的字段；词形还原所需 exchange 原样保留。
#
# 用法：python3 scripts/gen-dict.py /path/to/ecdict.csv
# 上游：https://github.com/skywind3000/ECDICT（MIT/无许可声明见仓库）。

import csv
import json
import sys
import re

TOP_N = 3000
WORD_RE = re.compile(r"^[a-z][a-z'-]*$")


def main(src: str, dst: str) -> None:
    rows = []
    with open(src, newline='', encoding='utf-8') as f:
        for row in csv.DictReader(f):
            word = (row.get('word') or '').strip().lower()
            frq = row.get('frq') or '0'
            if not word or not WORD_RE.match(word):
                continue
            try:
                frq_n = int(frq)
            except ValueError:
                continue
            if frq_n <= 0:
                continue
            translation = (row.get('translation') or '').replace('\\n', '\n').strip()
            if not translation:
                continue
            rows.append({
                'word': word,
                'phonetic': (row.get('phonetic') or '').strip(),
                'translation': translation,
                'pos': (row.get('pos') or '').strip(),
                'tag': (row.get('tag') or '').strip(),
                'exchange': (row.get('exchange') or '').strip(),
                'frq': frq_n,
            })
    rows.sort(key=lambda r: r['frq'])
    slim = rows[:TOP_N]
    for r in slim:
        del r['frq']
    with open(dst, 'w', encoding='utf-8') as f:
        json.dump(slim, f, ensure_ascii=False, separators=(',', ':'))
    print(f'wrote {len(slim)} entries to {dst}')


if __name__ == '__main__':
    if len(sys.argv) != 3:
        print('usage: gen-dict.py <ecdict.csv> <out.json>', file=sys.stderr)
        sys.exit(1)
    main(sys.argv[1], sys.argv[2])
