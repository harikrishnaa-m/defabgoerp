import openpyxl, os, re

folder = r"internal\migration\Defab Thrippunithura"
files = [f for f in os.listdir(folder) if f.endswith(".xlsx")]

seen = set()  # (cat, item_name, code) – same dedup key as Go importer

for fname in sorted(files):
    path = os.path.join(folder, fname)
    wb = openpyxl.load_workbook(path, data_only=True)
    cat = re.sub(r"\.xlsx$", "", fname, flags=re.IGNORECASE).strip()
    file_count = 0
    for sheet in wb.worksheets:
        rows = list(sheet.iter_rows(values_only=True))
        col_item = col_code = -1
        hdr_idx = None
        # detectColumns looks only in first 3 rows – mirror Go behaviour
        for i, row in enumerate(rows[:3]):
            vals = [str(c).strip().upper() if c else "" for c in row]
            for j, v in enumerate(vals):
                # Go: strings.Contains(upper, "ITEM") || strings.Contains(upper, "ITEAM")
                if col_item < 0 and ("ITEM" in v or "ITEAM" in v):
                    col_item = j
                if col_code < 0 and v == "CODE":
                    col_code = j
            if col_code >= 0:
                hdr_idx = i
                break
        if hdr_idx is None:
            print(f"  {fname} / {sheet.title}: header not found (skipped – same as Go)")
            continue
        # Go fallback: if itemIdx not found and codeIdx == 1, use col 2
        if col_item < 0:
            col_item = 2 if col_code == 1 else 1
        for row in rows[hdr_idx + 1:]:
            try:
                item = str(row[col_item]).strip() if col_item < len(row) and row[col_item] else ""
                code_val = row[col_code] if col_code < len(row) else None
                if not code_val or str(code_val).strip() in ("", "None"):
                    continue
                code = int(float(str(code_val)))
                if code <= 0:
                    continue
                key = (cat.lower(), item.lower(), code)
                seen.add(key)
                file_count += 1
            except Exception:
                pass
    print(f"{fname:40s}  rows={file_count}")

print(f"\nUnique (cat + item + code) variants : {len(seen)}")
print(f"API reported                         : 3504")
print(f"Match: {len(seen) == 3504}")
