"""Build a standalone, synthetic-only HTML fixture for visual review."""
from pathlib import Path
import re
root = Path(__file__).resolve().parents[1]
web = root / 'internal/desktop/web'
html = (web / 'index.html').read_text()
html = html.replace("style-src 'self'", "style-src 'self' 'unsafe-inline'")
html = re.sub(r'<link rel="stylesheet" href="style.css"\s*/?>', lambda _: '<style>' + (web / 'style.css').read_text() + '</style>', html)
html = html.replace('<script type="module" src="app.js"></script>', '')
model = (web / 'model.js').read_text().replace('export ', '')
app = re.sub(r'^import [\s\S]*?;\n', '', (web / 'app.js').read_text(), count=1)
fixture = (root / 'internal/desktop/testdata/fixture.js').read_text()
html = html.replace('</body>', '<script>' + fixture + model + app + '</script></body>')
out = root / 'dist/ui-preview'
out.mkdir(parents=True, exist_ok=True)
(out / 'index.html').write_text(html)
(out / 'icon.svg').write_text((web / 'icon.svg').read_text())
print(out / 'index.html')
