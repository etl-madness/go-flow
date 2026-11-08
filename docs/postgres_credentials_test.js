const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const docsDir = __dirname;
let data;

test.before(() => {
    const bundle = fs.readFileSync(path.join(docsDir, 'docs_bundle.js'), 'utf8');
    const start = bundle.indexOf('window.FLOW_DOCS_BUNDLE = ');
    assert.notEqual(start, -1, 'offline bundle declaration not found');
    data = JSON.parse(bundle.slice(start + 'window.FLOW_DOCS_BUNDLE = '.length).trim().replace(/;$/, ''));
});

function assertNoLiteralPostgresPasswords(content, name) {
    for (const [index, line] of content.split(/\r?\n/).entries()) {
        const normalized = line.replace(/\$\(\[uri\]::EscapeDataString\(\$env:PG_(?:USER|PASSWORD)\)\)/gi, '{{ENV}}');
        const uri = /\bpostgres(?:ql)?:\/\/([^/\s"'`<>@:]+):([^/\s"'`<>@]+)@/gi;
        for (const match of normalized.matchAll(uri)) {
            assert.ok(/^(?:\{\{|\$\{|\$\(|\{)/.test(match[2]), `${name}:${index + 1} has a literal PostgreSQL URI password`);
        }
        if (/\bport=5432\b/i.test(line)) {
            for (const match of line.matchAll(/\b(?:password|pwd)=([^\s;"'`]+)/gi)) {
                assert.ok(/^(?:\{\{|\$\{|\$\()/.test(match[1]), `${name}:${index + 1} has a literal PostgreSQL password`);
            }
        }
    }
}

test('offline bundle and source documents do not contain literal PostgreSQL example passwords', () => {
    for (const [name, content] of Object.entries(data)) {
        assertNoLiteralPostgresPasswords(content, `bundle:${name}`);
        const source = path.join(docsDir, name);
        assertNoLiteralPostgresPasswords(fs.readFileSync(source, 'utf8'), name);
    }
});

test('environment-backed PostgreSQL example is included in the offline bundle', () => {
    const guide = data['database_connection_strings.md'];
    assert.ok(guide.includes('os.environ["PG_USER"]'));
    assert.ok(guide.includes('os.environ["PG_PASSWORD"]'));
    assert.ok(guide.includes('quote(os.environ["PG_PASSWORD"], safe="")'));
});
