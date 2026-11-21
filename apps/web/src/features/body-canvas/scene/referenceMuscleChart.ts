import {
  Color,
  type BufferAttribute,
  type BufferGeometry,
  Float32BufferAttribute,
  FrontSide,
  type InterleavedBufferAttribute,
  MeshStandardMaterial,
  Vector3,
} from "three";

/** Classic anatomical-chart red; tendons and aponeuroses a muted rose so hands/feet do not read white. */
export const MUSCLE_COLOR = "#a3271f";
export const TENDON_COLOR = "#b86259";

/** Principal axis of a muscle part (its fibre direction, roughly), by power iteration on the covariance. */
function principalAxis(
  position: BufferAttribute | InterleavedBufferAttribute,
): Vector3 {
  const n = position.count;
  let mx = 0,
    my = 0,
    mz = 0;
  for (let i = 0; i < n; i++) {
    mx += position.getX(i);
    my += position.getY(i);
    mz += position.getZ(i);
  }
  mx /= n;
  my /= n;
  mz /= n;
  let xx = 0,
    xy = 0,
    xz = 0,
    yy = 0,
    yz = 0,
    zz = 0;
  for (let i = 0; i < n; i++) {
    const x = position.getX(i) - mx,
      y = position.getY(i) - my,
      z = position.getZ(i) - mz;
    xx += x * x;
    xy += x * y;
    xz += x * z;
    yy += y * y;
    yz += y * z;
    zz += z * z;
  }
  const axis = new Vector3(0.3, 1, 0.2).normalize();
  for (let k = 0; k < 24; k++) {
    axis.set(
      xx * axis.x + xy * axis.y + xz * axis.z,
      xy * axis.x + yy * axis.y + yz * axis.z,
      xz * axis.x + yz * axis.y + zz * axis.z,
    );
    if (axis.lengthSq() < 1e-20) return new Vector3(0, 1, 0);
    axis.normalize();
  }
  return axis;
}

/** Per-vertex fibre direction and tissue (0 muscle, 1 tendon) for the chart shading. */
export function addMuscleAttributes(
  geometry: BufferGeometry,
  tendon: boolean,
): void {
  const position = geometry.getAttribute("position");
  const axis = tendon ? new Vector3(0, 1, 0) : principalAxis(position);
  const fiber = new Float32Array(position.count * 3);
  const tissue = new Float32Array(position.count).fill(tendon ? 1 : 0);
  for (let i = 0; i < position.count; i++) {
    fiber[i * 3] = axis.x;
    fiber[i * 3 + 1] = axis.y;
    fiber[i * 3 + 2] = axis.z;
  }
  geometry.setAttribute("fiber", new Float32BufferAttribute(fiber, 3));
  geometry.setAttribute("tissue", new Float32BufferAttribute(tissue, 1));
}

/**
 * Red muscle with fine fibre striations running along each muscle's main axis,
 * pale tendons. Striations fade out with distance (fwidth) so they never alias.
 */
export function createMuscleMaterial(): MeshStandardMaterial {
  const material = new MeshStandardMaterial({
    color: MUSCLE_COLOR,
    roughness: 0.52,
    metalness: 0,
    side: FrontSide,
  });
  material.onBeforeCompile = (shader) => {
    shader.uniforms.tendonColor = { value: new Color(TENDON_COLOR) };
    shader.vertexShader = shader.vertexShader
      .replace(
        "#include <common>",
        `#include <common>
attribute vec3 fiber;
attribute float tissue;
varying float vStripe;
varying float vBundle;
varying float vTissue;`,
      )
      .replace(
        "#include <begin_vertex>",
        `#include <begin_vertex>
vec3 bsAcross = cross(normalize(fiber), normalize(objectNormal));
bsAcross = dot(bsAcross, bsAcross) > 1e-6 ? normalize(bsAcross) : vec3(1.0, 0.0, 0.0);
vStripe = dot(position, bsAcross);
vBundle = dot(position, normalize(fiber));
vTissue = tissue;`,
      );
    shader.fragmentShader = shader.fragmentShader
      .replace(
        "#include <common>",
        `#include <common>
uniform vec3 tendonColor;
varying float vStripe;
varying float vBundle;
varying float vTissue;`,
      )
      .replace(
        "#include <color_fragment>",
        `#include <color_fragment>
{
  float fine = vStripe * 1400.0;
  float coarse = vStripe * 260.0 + sin(vBundle * 40.0) * 1.4;
  float fadeFine = clamp(1.0 - fwidth(fine) / 2.2, 0.0, 1.0);
  float fadeCoarse = clamp(1.0 - fwidth(coarse) / 2.2, 0.0, 1.0);
  float striation = 1.0 - (0.13 * fadeFine * (0.5 + 0.5 * sin(fine)) + 0.16 * fadeCoarse * smoothstep(0.55, 1.0, 0.5 + 0.5 * sin(coarse)));
  float muscle = 1.0 - smoothstep(0.25, 0.75, vTissue);
  diffuseColor.rgb = mix(tendonColor, diffuseColor.rgb * striation, muscle);
}`,
      );
  };
  material.customProgramCacheKey = () => "bodysense-muscle-chart-v1";
  return material;
}
