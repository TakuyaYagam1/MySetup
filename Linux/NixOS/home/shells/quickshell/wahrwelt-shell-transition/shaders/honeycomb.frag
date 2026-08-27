// Adapted from Noctalia's wp_honeycomb.frag.
// Copyright (c) 2025 noctalia-dev. Licensed under the MIT License.
// The retained notice is in LICENSE-Noctalia-MIT.txt.
#version 450

layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;

layout(binding = 1) uniform sampler2D source;

layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    float progress;
    float cellSize;
    float aspectRatio;
    float centerX;
    float centerY;
} ubuf;

vec2 hexRound(vec2 axial) {
    float x = axial.x;
    float z = axial.y;
    float y = -x - z;

    float rx = round(x);
    float ry = round(y);
    float rz = round(z);

    float dx = abs(rx - x);
    float dy = abs(ry - y);
    float dz = abs(rz - z);

    if (dx > dy && dx > dz) {
        rx = -ry - rz;
    } else if (dy > dz) {
        ry = -rx - rz;
    } else {
        rz = -rx - ry;
    }

    return vec2(rx, rz);
}

void main() {
    vec2 uv = qt_TexCoord0;
    vec2 aspectUv = vec2(uv.x * ubuf.aspectRatio, uv.y);
    float size = max(ubuf.cellSize, 0.01);

    float q = (aspectUv.x * (2.0 / 3.0)) / size;
    float r = ((-aspectUv.x / 3.0) + (sqrt(3.0) / 3.0) * aspectUv.y) / size;
    vec2 hex = hexRound(vec2(q, r));

    vec2 hexCenter = vec2(
        size * (3.0 / 2.0) * hex.x,
        size * sqrt(3.0) * (hex.y + 0.5 * hex.x)
    );
    vec2 origin = vec2(ubuf.centerX * ubuf.aspectRatio, ubuf.centerY);
    float distanceFromCenter = distance(hexCenter, origin);

    float maxDistanceX = max(
        ubuf.centerX * ubuf.aspectRatio,
        (1.0 - ubuf.centerX) * ubuf.aspectRatio
    );
    float maxDistanceY = max(ubuf.centerY, 1.0 - ubuf.centerY);
    float maxDistance = length(vec2(maxDistanceX, maxDistanceY));
    float softEdge = 0.15 * maxDistance;
    float radius = -softEdge + ubuf.progress * (maxDistance + 2.0 * softEdge);

    float frozenAlpha = smoothstep(
        radius - softEdge,
        radius + softEdge,
        distanceFromCenter
    );
    fragColor = texture(source, uv) * frozenAlpha * ubuf.qt_Opacity;
}
