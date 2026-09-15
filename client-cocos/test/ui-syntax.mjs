import fs from 'node:fs';
import path from 'node:path';
import ts from 'typescript';

const root = path.resolve('assets/scripts');
const files = [];

function walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full);
    else if (entry.isFile() && entry.name.endsWith('.ts')) files.push(full);
  }
}

walk(root);

let errorCount = 0;
for (const file of files) {
  const source = fs.readFileSync(file, 'utf8');
  const result = ts.transpileModule(source, {
    fileName: file,
    reportDiagnostics: true,
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.ESNext,
      experimentalDecorators: true,
      useDefineForClassFields: false,
    },
  });

  for (const diagnostic of result.diagnostics ?? []) {
    if (diagnostic.category !== ts.DiagnosticCategory.Error) continue;
    errorCount++;
    const message = ts.flattenDiagnosticMessageText(diagnostic.messageText, '\n');
    if (diagnostic.file && diagnostic.start !== undefined) {
      const pos = diagnostic.file.getLineAndCharacterOfPosition(diagnostic.start);
      console.error(`${file}:${pos.line + 1}:${pos.character + 1} ${message}`);
    } else {
      console.error(`${file}: ${message}`);
    }
  }
}

if (errorCount > 0) {
  console.error(`Cocos TypeScript syntax smoke failed: ${errorCount} error(s)`);
  process.exit(1);
}

console.log(`Cocos TypeScript syntax smoke passed: ${files.length} file(s)`);
