import re, subprocess, tempfile, os

with open("/home/worker1/dexbot/web_output/portfolio.html", "r") as f:
    html = f.read()

# Extract first script block
scripts = list(re.finditer(r"<script>(.*?)</script>", html, re.DOTALL))
if not scripts:
    print("NO SCRIPT BLOCKS FOUND")
    exit(1)

js = scripts[0].group(1)

# Write to temp file
tf = tempfile.NamedTemporaryFile(mode='w', suffix='.js', delete=False)
tf.write(js)
tf.close()

# Try running through node
result = subprocess.run(["node", "--check", tf.name], capture_output=True, text=True, timeout=5)
print("NODE CHECK:", result.stderr.strip() if result.stderr else "PASS")
print("Exit:", result.returncode)

# Also check specific patterns
lines = js.split("\n")
for i, line in enumerate(lines, 1):
    # Look for lines that end mid-string
    stripped = line.strip()
    if stripped and not stripped.startswith("//") and not stripped.startswith("/*"):
        dq = stripped.count('"')
        if dq % 2 != 0 and not stripped.endswith(","):
            print(f"Line {i}: possible broken string: {stripped[:80]}")

os.unlink(tf.name)
