package io.github.xaviiromf.cliampx.ui

/**
 * The AGSL sources of the backgrounds (Backgrounds.kt). Every shader gets
 * the same uniforms: the window size res and this view's origin in it (so
 * a sheet lines up with the app behind it), t in seconds, dp in pixels,
 * and six colors of the theme. paint gets the pixel fc, the same point in
 * dp p, and uv in 0..1 of the window.
 */
private const val PRELUDE = """
uniform float2 res;
uniform float2 origin;
uniform float t;
uniform float dp;
layout(color) uniform half4 bg;
layout(color) uniform half4 accent;
layout(color) uniform half4 fg;
layout(color) uniform half4 green;
layout(color) uniform half4 yellow;
layout(color) uniform half4 red;

float hash(float2 q) {
    q = fract(q * float2(123.34, 456.21));
    q += dot(q, q + 45.32);
    return fract(q.x * q.y);
}

float noise(float2 q) {
    float2 i = floor(q);
    float2 f = fract(q);
    f = f * f * (3.0 - 2.0 * f);
    return mix(mix(hash(i), hash(i + float2(1.0, 0.0)), f.x), mix(hash(i + float2(0.0, 1.0)), hash(i + float2(1.0, 1.0)), f.x), f.y);
}

float fbm(float2 q) {
    float v = 0.0;
    float a = 0.5;
    for (int i = 0; i < 4; i++) {
        v += a * noise(q);
        q *= 2.0;
        a *= 0.5;
    }
    return v;
}

float bayer2(float2 a) {
    a = floor(a);
    return fract(a.x / 2.0 + a.y * a.y * 0.75);
}

float bayer4(float2 a) {
    return bayer2(0.5 * a) * 0.25 + bayer2(a);
}
"""

private const val MAIN = """
half4 main(float2 xy) {
    float2 fc = xy + origin;
    return half4(clamp(paint(fc, fc / dp, fc / res), 0.0, 1.0), 1.0);
}
"""

private val BODIES = mapOf(
    Background.Crt to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float line = 0.5 + 0.5 * sin(p.y * 3.14159);
    float r0 = (fract(uv.y * 0.5 - t * 0.05) - 0.5) * 8.0;
    float roll = exp(-r0 * r0);
    float2 d = uv - 0.5;
    float vig = 1.0 - dot(d, d) * 1.2;
    half3 c = mix(bg.rgb, accent.rgb, 0.35 * line + 0.4 * roll);
    return c * vig;
}""",
    Background.Waves to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float h = res.y / dp;
    float w = 0.0;
    float glow = 0.0;
    for (int i = 0; i < 5; i++) {
        float fi = float(i);
        float y = h * (0.15 + 0.17 * fi) + 22.0 * sin(p.x * 0.02 * (1.0 + fi * 0.3) + t * (0.5 + 0.15 * fi) + fi * 1.7);
        float d = abs(p.y - y);
        w += smoothstep(1.6, 0.0, d) * (1.0 - fi * 0.12);
        glow += smoothstep(26.0, 0.0, d) * 0.12;
    }
    half3 c = mix(accent.rgb, green.rgb, uv.x);
    return mix(bg.rgb, c, clamp(w + glow, 0.0, 1.0));
}""",
    Background.Cyberpunk to """
half3 paint(float2 fc, float2 p, float2 uv) {
    half3 base = mix(bg.rgb, mix(red.rgb, accent.rgb, uv.x * 0.5 + uv.y * 0.5), 0.3);
    float lines = step(0.96, fract(p.y / 28.0 + t * 0.04));
    float col = floor(p.x / 6.0);
    float streak = step(0.97, hash(float2(col, 0.0))) * pow(fract(-p.y / 260.0 + t * (0.3 + hash(float2(col, 1.0)))), 8.0);
    return base + accent.rgb * lines * 0.3 + yellow.rgb * streak * 0.8;
}""",
    Background.Retro to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float hz = 0.58;
    if (uv.y < hz) {
        half3 sky = mix(bg.rgb, mix(bg.rgb, red.rgb, 0.55), uv.y / hz);
        float r = min(res.x, res.y) / dp * 0.3;
        float2 c = float2(res.x * 0.5, res.y * hz) / dp - float2(0.0, r * 0.35);
        float d = length(p - c);
        float sun = smoothstep(r, r - 1.5, d);
        float band = clamp((p.y - (c.y - r)) / (2.0 * r), 0.0, 1.0);
        float cut = band < 0.45 ? 1.0 : step(band * 0.6, fract(p.y / 9.0));
        return mix(sky, mix(yellow.rgb, red.rgb, band), sun * cut);
    }
    float y = uv.y - hz;
    float z = 0.12 / y;
    float2 g = float2((uv.x - 0.5) * z * 9.0, z * 2.0 + t * 0.8);
    float2 f = abs(fract(g) - 0.5);
    float line = smoothstep(0.42, 0.5, max(f.x, f.y));
    float fade = smoothstep(0.0, 0.12, y);
    return mix(mix(bg.rgb, red.rgb, 0.12), accent.rgb, line * fade);
}""",
    Background.Glitch to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float tb = floor(t * 6.0);
    float row = floor(p.y / 14.0);
    float on = step(0.86, hash(float2(row, tb)));
    float shift = (hash(float2(row, tb + 1.0)) - 0.5) * 60.0 * on;
    float bar = step(0.5, fract((p.x + shift) / (30.0 + 60.0 * hash(float2(row, 7.0)))));
    half3 c = bg.rgb;
    c = mix(c, accent.rgb, on * bar * 0.45);
    c = mix(c, red.rgb, on * (1.0 - bar) * 0.3 * step(0.5, hash(float2(row, tb + 2.0))));
    c += fg.rgb * 0.08 * hash(float2(floor(p.y), floor(t * 30.0)));
    return c * (0.9 + 0.1 * (0.5 + 0.5 * sin(p.y * 3.14159)));
}""",
    Background.Matrix to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float2 size = float2(10.0, 14.0);
    float2 cell = floor(p / size);
    float2 inC = fract(p / size);
    float rows = res.y / dp / size.y;
    float head = fract(t * (0.04 + hash(float2(cell.x, 1.0)) * 0.08) + hash(float2(cell.x, 2.0)));
    float d = fract(head - cell.y / rows);
    float trail = pow(1.0 - d, 5.0);
    float2 g = floor(inC * float2(3.0, 5.0));
    float bit = step(0.45, hash(cell * float2(3.1, 7.7) + g * 1.3 + floor(t * 1.5 + hash(cell) * 9.0)));
    float inside = step(0.15, inC.x) * step(inC.x, 0.85) * step(0.1, inC.y) * step(inC.y, 0.9);
    float v = trail * bit * inside;
    half3 c = mix(bg.rgb, green.rgb, v);
    return mix(c, fg.rgb, step(d, 0.015) * bit * inside);
}""",
    Background.Stars to """
half3 paint(float2 fc, float2 p, float2 uv) {
    half3 c = bg.rgb;
    for (int i = 0; i < 3; i++) {
        float fi = float(i);
        float s = 34.0 + fi * 26.0;
        float2 q = p / s + float2(t * 0.03 / (fi + 1.0), t * 0.01);
        float2 id = floor(q) + fi * 17.0;
        float2 f = fract(q) - 0.5;
        float2 off = float2(hash(id + 1.7), hash(id + 3.1)) - 0.5;
        float d = length(f - off * 0.7) * s;
        float tw = 0.55 + 0.45 * sin(t * 2.0 + hash(id) * 40.0);
        float star = smoothstep(2.4 - fi * 0.4, 0.0, d) * step(0.45, hash(id + 9.0)) * tw;
        star += smoothstep(7.0, 0.0, d) * 0.12 * step(0.9, hash(id + 9.0));
        c = mix(c, mix(fg.rgb, accent.rgb, hash(id + 5.0)), clamp(star, 0.0, 1.0));
    }
    return c;
}""",
    Background.Plasma to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float2 q = p / 70.0;
    float v = sin(q.x + t * 0.5) + sin(q.y * 1.3 - t * 0.4) + sin((q.x + q.y) * 0.7 + t * 0.3)
        + sin(length(q - float2(3.0, 5.0) + float2(sin(t * 0.2), cos(t * 0.3)) * 2.0));
    v = v * 0.25 + 0.5;
    half3 c = mix(mix(accent.rgb, red.rgb, smoothstep(0.0, 0.5, v)), green.rgb, smoothstep(0.5, 1.0, v));
    return mix(bg.rgb, c, 0.6);
}""",
    Background.Aurora to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float x = p.x / 140.0;
    float n = fbm(float2(x * 2.0 + t * 0.05, t * 0.07));
    float y = uv.y;
    float band = smoothstep(0.0, 0.2, y) * smoothstep(0.7, 0.25, y + n * 0.35 - 0.15);
    float rays = 0.55 + 0.45 * sin(p.x / 9.0 + n * 12.0 + t * 0.3);
    float a = band * rays * (0.5 + 0.7 * fbm(float2(x * 4.0, y * 3.0 - t * 0.1)));
    half3 c = mix(green.rgb, accent.rgb, smoothstep(0.1, 0.6, y));
    return mix(bg.rgb, c, clamp(a, 0.0, 1.0));
}""",
    Background.Vhs to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float k = (uv.y - fract(t * 0.09)) * 25.0;
    float trk = exp(-k * k);
    float n = hash(float2(floor(p.y / 2.0), floor(t * 24.0)));
    float x = p.x + sin(p.y * 0.05 + t * 3.0) * 2.0 + trk * 24.0 * (n - 0.5);
    float stripes = 0.5 + 0.5 * sin(x / 18.0 + t * 0.6);
    half3 c = mix(bg.rgb, accent.rgb, 0.18 + 0.14 * stripes);
    c = mix(c, red.rgb, 0.2 * step(0.5, fract((x + 3.0) / 40.0)));
    return c + fg.rgb * (trk * 0.5 * hash(float2(floor(p.x * 0.5) + t, p.y)) + 0.06 * hash(fc + fract(t)));
}""",
    Background.Dither to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float2 cell = floor(p / 3.0);
    float g = 0.5 + 0.5 * sin(cell.x * 0.04 + t * 0.3) * cos(cell.y * 0.03 - t * 0.2);
    g = g * 0.6 + (1.0 - uv.y) * 0.35;
    float on = step(bayer4(cell), g * 0.75);
    return mix(bg.rgb, accent.rgb, on * 0.6);
}""",
    Background.Tron to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float s = 40.0;
    float2 g = p / s;
    float2 f = abs(fract(g + 0.5) - 0.5) * s;
    float m = min(f.x, f.y);
    float line = smoothstep(1.2, 0.0, m) * 0.55 + smoothstep(9.0, 0.0, m) * 0.15;
    float2 id = floor(g + 0.5);
    float px = fract(p.x / (s * 8.0) - t * 0.12 + hash(float2(id.y, 1.0)));
    float py = fract(p.y / (s * 8.0) - t * 0.1 + hash(float2(id.x, 2.0)));
    float pulse = smoothstep(0.06, 0.0, abs(px - 0.5)) * smoothstep(2.5, 0.0, f.y) * step(0.5, hash(float2(id.y, 3.0)))
        + smoothstep(0.06, 0.0, abs(py - 0.5)) * smoothstep(2.5, 0.0, f.x) * step(0.5, hash(float2(id.x, 4.0)));
    return mix(bg.rgb + accent.rgb * line, fg.rgb, clamp(pulse, 0.0, 1.0));
}""",
    Background.Bokeh to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float2 size = res / dp;
    half3 c = bg.rgb;
    for (int i = 0; i < 12; i++) {
        float fi = float(i);
        float r = 24.0 + 56.0 * hash(float2(fi, 4.0));
        float y = fract(hash(float2(fi, 2.0)) - t * 0.012 * (0.4 + hash(float2(fi, 3.0)))) * (size.y + 2.0 * r) - r;
        float x = hash(float2(fi, 1.0)) * size.x + sin(t * 0.2 + fi) * 12.0;
        float d = length(p - float2(x, y));
        float a = smoothstep(r, r * 0.8, d) * 0.3 + smoothstep(2.0, 0.0, abs(d - r + 2.0)) * 0.15;
        half3 col = mix(mix(accent.rgb, green.rgb, step(0.4, hash(float2(fi, 5.0)))), yellow.rgb, step(0.8, hash(float2(fi, 5.0))));
        c = mix(c, col, a);
    }
    return c;
}""",
    Background.Circuit to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float s = 26.0;
    float2 id = floor(p / s);
    float2 f = fract(p / s);
    float h = hash(id);
    float d = h < 0.4 ? abs(f.y - 0.5) : (h < 0.8 ? abs(f.x - 0.5) : abs(f.x - f.y) * 0.707);
    float on = step(0.3, hash(id + 9.0));
    float line = smoothstep(0.07, 0.035, d) * on;
    float r = length(f - 0.5);
    float pad = smoothstep(0.17, 0.13, r) * smoothstep(0.05, 0.09, r) * step(0.82, hash(id + 4.0));
    float sig = pow(fract(hash(id + 2.0) + t * 0.25 + (h < 0.4 ? f.x : f.y) * 0.2), 12.0);
    half3 c = mix(bg.rgb, green.rgb, clamp(line * 0.55 + pad * 0.7, 0.0, 1.0));
    return mix(c, accent.rgb, line * sig);
}""",
    Background.Topo to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float n = fbm(p / 200.0 + float2(t * 0.012, t * 0.008));
    float v = n * 14.0;
    float f = min(fract(v), 1.0 - fract(v));
    float major = step(0.5, fract(floor(v + 0.5) / 4.0 + 0.125)) * 0.5 + 0.5;
    float line = smoothstep(0.09, 0.02, f) * major;
    return mix(bg.rgb, mix(accent.rgb, green.rgb, n), line * 0.75);
}""",
    Background.Grain to """
half3 paint(float2 fc, float2 p, float2 uv) {
    float g = hash(fc + fract(t * 7.0) * 100.0) - 0.5;
    float2 d = uv - 0.5;
    float vig = 1.0 - dot(d, d) * 0.9;
    float leak = 0.5 + 0.5 * sin(t * 0.2 + uv.x * 2.0);
    half3 c = mix(bg.rgb, mix(accent.rgb, red.rgb, uv.y), 0.18 * leak * (1.0 - uv.y));
    return c * vig + fg.rgb * g * 0.3;
}""",
)

/** The whole AGSL program of b, or null for Sólido. */
fun shaderSource(b: Background): String? = BODIES[b]?.let { PRELUDE + it + MAIN }
