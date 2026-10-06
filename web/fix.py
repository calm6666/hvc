import re, glob, os

base = r'D:\hilihili\server\hvc\web\src\views'
count = 0

for path in glob.glob(f'{base}/**/*.vue', recursive=True):
    with open(path, 'r', encoding='utf-8') as f:
        c = f.read()
    old = c

    # 1. Remove useTable from imports
    c = re.sub(r"import \{([^}]*)\} from '@/components/Table';",
               lambda m: "import {" + ", ".join(
                   s.strip() for s in m.group(1).split(',')
                   if s.strip() and s.strip() != 'useTable'
               ) + " } from '@/components/Table';", c)

    # 2. BasicModal -> basicModal
    c = c.replace('import { BasicModal,', 'import { basicModal,')

    # 3. Template: <BasicTable @register="registerTable"> -> <BasicTable ... ref="actionRef">
    c = c.replace('@register="registerTable"', 'ref="actionRef"')

    # 4. Replace useTable call: const [registerTable, { reload }] = useTable({...single-line...});
    def replace_use_table(m):
        body = m.group(1)  # { columns, request: ..., rowKey: '...' }
        return 'const actionRef = ref();\n  function reload() { actionRef.value?.reload(); }\n  // was: const [_r, {reload}] = useTable(' + body + ')'
    c = re.sub(r"const\s+\[registerTable,\s*\{[^}]*\}\]\s*=\s*useTable\((\{[^}]+\})\);", replace_use_table, c)

    if c != old:
        with open(path, 'w', encoding='utf-8') as f:
            f.write(c)
        print(f'FIXED: {path}')
        count += 1

print(f'\nFixed {count} files')
