import { useEffect, useRef } from "react";
import { useFrame, useThree } from "@react-three/fiber";
import { Matrix4, Vector3, InstancedMesh } from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import type { BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import { DEFAULT_BODY_LAYERS, getReferenceMotion } from "../model/bodyCanvas";
import {
  applyReferencePose,
  SKIN_COLOR,
  SKIN_OPACITY,
  SKIN_SELECTED_OPACITY,
} from "./referenceRig";
import { useSurfacePainting, type SurfaceAnchor } from "./useSurfacePainting";
import type { ReferenceCanvasContentProps } from "./types";

export function ReferenceCanvasContents(props: ReferenceCanvasContentProps) {
  const { rig } = props;
  const { camera, gl, invalidate, size } = useThree();
  const latest = useRef(props);
  latest.current = props;
  const controls = useRef<OrbitControls | null>(null);
  const phase = useRef(0);
  const reportedAt = useRef(0);
  const anchors = useRef<SurfaceAnchor[]>([]);
  const marks = useRef<InstancedMesh>(null);
  const scratch = useRef({
    a: new Vector3(),
    b: new Vector3(),
    c: new Vector3(),
    p: new Vector3(),
    normal: new Vector3(),
    m: new Matrix4(),
  });

  useEffect(() => {
    const orbit = new OrbitControls(camera, gl.domElement);
    orbit.enableDamping = true;
    orbit.dampingFactor = 0.12;
    orbit.minDistance = 1.15;
    orbit.maxDistance = 5.5;
    orbit.minPolarAngle = 0.15;
    orbit.maxPolarAngle = Math.PI * 0.86;
    orbit.target.set(0, 0.86, 0);
    orbit.update();
    const handleOrbitChange = () => invalidate();
    orbit.addEventListener("change", handleOrbitChange);
    controls.current = orbit;
    const visibility = () => {
      if (document.visibilityState === "visible") invalidate();
    };
    document.addEventListener("visibilitychange", visibility);
    const lost = (event: Event) => {
      event.preventDefault();
      latest.current.onContextLost();
    };
    gl.domElement.addEventListener("webglcontextlost", lost);
    return () => {
      orbit.removeEventListener("change", handleOrbitChange);
      orbit.dispose();
      controls.current = null;
      document.removeEventListener("visibilitychange", visibility);
      gl.domElement.removeEventListener("webglcontextlost", lost);
    };
  }, [camera, gl, invalidate]);

  // Penpot Home frames the reference body at ~3/4 of the full-height stage
  // (desktop y 105-775 of 900, mobile 155-700 of 844); the stage has no app
  // bar, so widen the lens rather than letting the figure fill the viewport.
  useEffect(() => {
    if (!("fov" in camera)) return;
    const fov = size.width / Math.max(size.height, 1) < 0.8 ? 47 : 42;
    if (camera.fov === fov) return;
    camera.fov = fov;
    camera.updateProjectionMatrix();
    invalidate();
  }, [camera, size.width, size.height, invalidate]);

  const lying = props.motion === "bridge";
  useEffect(() => {
    camera.position.set(
      lying ? 1.6 : 0.36,
      lying ? 1.5 : 0.94,
      lying ? 2.5 : 3.12,
    );
    controls.current?.target.set(0, lying ? 0.38 : 0.86, 0);
    controls.current?.update();
    invalidate();
  }, [camera, lying, props.resetKey, invalidate]);
  useEffect(() => {
    if (controls.current) controls.current.enabled = props.tool === "orbit";
  }, [props.tool]);
  useEffect(() => {
    phase.current = props.seek.phase;
    applyReferencePose(rig, props.motion, phase.current);
    invalidate();
  }, [props.seek.revision, props.seek.phase, props.motion, rig, invalidate]);
  useEffect(() => {
    anchors.current = [];
    if (marks.current) marks.current.count = 0;
    invalidate();
  }, [props.annotationEpoch, invalidate]);
  const layers = props.layers ?? DEFAULT_BODY_LAYERS;
  useEffect(() => {
    if (props.muscles) props.muscles.mesh.visible = layers.muscles;
    if (props.skeleton) props.skeleton.mesh.visible = layers.skeleton;
    invalidate();
  }, [
    props.muscles,
    props.skeleton,
    layers.muscles,
    layers.skeleton,
    invalidate,
  ]);
  useEffect(() => {
    const selected = new Set(props.selectedRegions);
    rig.meshes.forEach((mesh) => {
      const active = selected.has(mesh.userData.bodyRegionId as BodyRegionId);
      // With the skin layer off, a selected region keeps its skin so the pick stays visible.
      mesh.visible = layers.skin || active;
      mesh.material.color.set(
        active
          ? props.dark
            ? "#85bb9a"
            : "#78a48b"
          : props.dark
            ? "#c9d6cc"
            : SKIN_COLOR,
      );
      mesh.material.opacity = active ? SKIN_SELECTED_OPACITY : SKIN_OPACITY;
      mesh.material.emissive.set(active ? "#244c36" : "#000000");
      mesh.material.emissiveIntensity = active ? 0.18 : 0;
    });
    anchors.current = anchors.current.filter((anchor) =>
      selected.has(anchor.mesh.userData.bodyRegionId as BodyRegionId),
    );
    invalidate();
  }, [props.selectedRegions, props.dark, layers.skin, rig, invalidate]);

  useSurfacePainting({ camera, gl, rig, invalidate, latest, anchors });

  useFrame((state, delta) => {
    const current = latest.current;
    if (current.playing && document.visibilityState !== "hidden") {
      phase.current =
        (phase.current +
          (Math.min(delta, 0.05) * current.speed) /
            getReferenceMotion(current.motion).duration) %
        1;
      invalidate();
    }
    applyReferencePose(rig, current.motion, phase.current);
    if (controls.current?.update()) invalidate();
    if (state.clock.elapsedTime - reportedAt.current > 0.12) {
      current.onPhaseChange(phase.current);
      reportedAt.current = state.clock.elapsedTime;
    }
    if (marks.current) {
      marks.current.count = anchors.current.length;
      const s = scratch.current;
      anchors.current.forEach((anchor, index) => {
        anchor.mesh.getVertexPosition(anchor.triangle[0], s.a);
        anchor.mesh.getVertexPosition(anchor.triangle[1], s.b);
        anchor.mesh.getVertexPosition(anchor.triangle[2], s.c);
        s.p
          .copy(s.a)
          .multiplyScalar(anchor.barycentric.x)
          .addScaledVector(s.b, anchor.barycentric.y)
          .addScaledVector(s.c, anchor.barycentric.z);
        s.m.makeTranslation(s.p.x, s.p.y, s.p.z);
        marks.current!.setMatrixAt(index, s.m);
      });
      marks.current.instanceMatrix.needsUpdate = true;
    }
  });

  return (
    <>
      <hemisphereLight args={["#f6faf5", "#627668", 2.1]} />
      <directionalLight position={[-2, 3, 4]} intensity={3.8} color="#fffcf2" />
      <directionalLight position={[2, 2, -3]} intensity={2.4} color="#c9e4d3" />
      <primitive object={rig.group} dispose={null} />
      <instancedMesh
        ref={marks}
        args={[undefined, undefined, 160]}
        frustumCulled={false}
      >
        <sphereGeometry args={[0.007, 8, 6]} />
        <meshBasicMaterial color="#286b50" />
      </instancedMesh>
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.016, 0]}>
        <ringGeometry args={[0.35, 0.353, 80]} />
        <meshBasicMaterial
          color={props.dark ? "#719680" : "#a5b9a8"}
          transparent
          opacity={0.36}
        />
      </mesh>
    </>
  );
}
