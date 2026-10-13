import { Component, useEffect, useRef, useState, type ReactNode } from "react";
import { Canvas, useFrame, useThree } from "@react-three/fiber";
import {
  Color, Matrix4, Raycaster, Triangle, Vector2, Vector3,
  InstancedMesh, SRGBColorSpace, ACESFilmicToneMapping,
} from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";
import { getBodyRegionDefinition, type BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import { getReferenceMotion, type CanvasTool, type MotionId } from "../model/bodyCanvas";
import { applyReferencePose, buildReferenceRig, type ReferenceRig } from "../scene/referenceRig";
import {
  loadPinnedReferenceAtlasSurface,
  type ReferenceAtlasSurface,
} from "../scene/referenceAtlasLoader";
import { SurfacePicker } from "../scene/surfacePicker";

export interface BodyCanvasSceneProps {
  tool: CanvasTool;
  selectedRegions: readonly BodyRegionId[];
  motion: MotionId;
  playing: boolean;
  speed: number;
  seek: { phase: number; revision: number };
  resetKey: number;
  annotationEpoch: number;
  dark: boolean;
  onSelect: (regions: BodyRegionId[], append: boolean) => void;
  onGestureStart: () => void;
  onPhaseChange: (phase: number) => void;
  onTextFallback: () => void;
}

class SceneBoundary extends Component<{ children: ReactNode; fallback: ReactNode }, { failed: boolean }> {
  override state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  override render() { return this.state.failed ? this.props.fallback : this.props.children; }
}

interface SurfaceAnchor { mesh: ReferenceRig["meshes"][number]; triangle: [number, number, number]; barycentric: Vector3 }

function SceneContents(props: BodyCanvasSceneProps & { rig: ReferenceRig; cursor: HTMLDivElement | null; box: HTMLDivElement | null; onHover: (id: BodyRegionId | null) => void; onContextLost: () => void }) {
  const { rig } = props;
  const { camera, gl, invalidate } = useThree();
  const latest = useRef(props);
  latest.current = props;
  const controls = useRef<OrbitControls | null>(null);
  const picker = useRef<SurfacePicker | null>(null);
  const phase = useRef(0);
  const reportedAt = useRef(0);
  const anchors = useRef<SurfaceAnchor[]>([]);
  const marks = useRef<InstancedMesh>(null);
  const scratch = useRef({ a: new Vector3(), b: new Vector3(), c: new Vector3(), p: new Vector3(), normal: new Vector3(), m: new Matrix4() });

  useEffect(() => {
    const orbit = new OrbitControls(camera, gl.domElement);
    orbit.enableDamping = true; orbit.dampingFactor = .12;
    orbit.minDistance = 1.15; orbit.maxDistance = 5.5;
    orbit.minPolarAngle = .15; orbit.maxPolarAngle = Math.PI * .86;
    orbit.target.set(0, .86, 0); orbit.update();
    const handleOrbitChange = () => invalidate();
    orbit.addEventListener("change", handleOrbitChange);
    controls.current = orbit;
    const visibility = () => { if (document.visibilityState === "visible") invalidate(); };
    document.addEventListener("visibilitychange", visibility);
    const lost = (event: Event) => { event.preventDefault(); latest.current.onContextLost(); };
    gl.domElement.addEventListener("webglcontextlost", lost);
    return () => { orbit.removeEventListener("change", handleOrbitChange); orbit.dispose(); controls.current = null; document.removeEventListener("visibilitychange", visibility); gl.domElement.removeEventListener("webglcontextlost", lost); };
  }, [camera, gl, invalidate]);

  const lying = props.motion === "bridge";
  useEffect(() => {
    camera.position.set(lying ? 1.6 : .36, lying ? 1.5 : .94, lying ? 2.5 : 3.12);
    controls.current?.target.set(0, lying ? .38 : .86, 0);
    controls.current?.update(); invalidate();
  }, [camera, lying, props.resetKey, invalidate]);
  useEffect(() => { if (controls.current) controls.current.enabled = props.tool === "orbit"; }, [props.tool]);
  useEffect(() => { phase.current = props.seek.phase; applyReferencePose(rig, props.motion, phase.current); invalidate(); }, [props.seek.revision, props.seek.phase, props.motion, rig, invalidate]);
  useEffect(() => { anchors.current = []; if (marks.current) marks.current.count = 0; invalidate(); }, [props.annotationEpoch, invalidate]);
  useEffect(() => {
    const selected = new Set(props.selectedRegions);
    rig.meshes.forEach((mesh) => {
      const active = selected.has(mesh.userData.bodyRegionId as BodyRegionId);
      mesh.material.color.set(active ? (props.dark ? "#85bb9a" : "#78a48b") : (props.dark ? "#adbeb0" : "#b5c5b9"));
      mesh.material.emissive.set(active ? "#244c36" : "#000000");
      mesh.material.emissiveIntensity = active ? .18 : 0;
    });
    anchors.current = anchors.current.filter((anchor) => selected.has(anchor.mesh.userData.bodyRegionId as BodyRegionId));
    invalidate();
  }, [props.selectedRegions, props.dark, rig, invalidate]);

  useEffect(() => {
    const idPicker = new SurfacePicker(gl, rig); picker.current = idPicker;
    const canvas = gl.domElement;
    const ray = new Raycaster();
    let drag: { pointer: number; x: number; y: number; lastX: number; lastY: number; distance: number; ids: Set<BodyRegionId>; append: boolean; anchorStart: number } | null = null;
    let lastPaint = 0;
    const local = (event: PointerEvent) => { const r = canvas.getBoundingClientRect(); return { x: event.clientX - r.left, y: event.clientY - r.top, r }; };
    const pass = () => { const r = canvas.getBoundingClientRect(); rig.skeleton.update(); idPicker.render(camera, r.width, r.height); };
    const hideOverlay = () => { if (latest.current.cursor) latest.current.cursor.style.display = "none"; if (latest.current.box) latest.current.box.style.display = "none"; };
    const addAnchor = (x: number, y: number) => {
      const region = idPicker.read({ x, y, width: 1, height: 1 })[0];
      const mesh = rig.meshes.find((item) => item.userData.bodyRegionId === region);
      if (!mesh || anchors.current.length >= 160) return;
      const r = canvas.getBoundingClientRect();
      ray.setFromCamera(new Vector2(x / r.width * 2 - 1, 1 - y / r.height * 2), camera);
      const hit = ray.intersectObject(mesh, false)[0];
      if (!hit?.face) return;
      const { a, b, c } = hit.face;
      const pointA = mesh.getVertexPosition(a, new Vector3()), pointB = mesh.getVertexPosition(b, new Vector3()), pointC = mesh.getVertexPosition(c, new Vector3());
      const barycentric = Triangle.getBarycoord(hit.point, pointA, pointB, pointC, new Vector3());
      if (barycentric) anchors.current.push({ mesh, triangle: [a, b, c], barycentric });
    };
    const paint = (x: number, y: number) => {
      pass();
      idPicker.read({ x: x - 17, y: y - 17, width: 34, height: 34 }, true).forEach((id) => drag?.ids.add(id));
      addAnchor(x, y); invalidate();
    };
    const down = (event: PointerEvent) => {
      if (event.button !== 0 || drag) return;
      const p = local(event);
      drag = { pointer: event.pointerId, x: p.x, y: p.y, lastX: p.x, lastY: p.y, distance: 0, ids: new Set(), append: event.shiftKey, anchorStart: anchors.current.length };
      if (latest.current.tool !== "orbit") { canvas.setPointerCapture(event.pointerId); latest.current.onGestureStart(); }
      if (latest.current.tool === "brush") { paint(p.x, p.y); lastPaint = performance.now(); }
    };
    const move = (event: PointerEvent) => {
      const p = local(event), current = latest.current;
      if (current.tool === "brush" && current.cursor) Object.assign(current.cursor.style, { display: "block", left: `${p.x}px`, top: `${p.y}px` });
      if (!drag || event.pointerId !== drag.pointer) return;
      drag.distance += Math.abs(p.x - drag.lastX) + Math.abs(p.y - drag.lastY); drag.lastX = p.x; drag.lastY = p.y;
      if (current.tool === "brush" && performance.now() - lastPaint > 30) { paint(p.x, p.y); lastPaint = performance.now(); }
      if (current.tool === "box" && current.box) Object.assign(current.box.style, { display: "block", left: `${Math.min(p.x, drag.x)}px`, top: `${Math.min(p.y, drag.y)}px`, width: `${Math.abs(p.x - drag.x)}px`, height: `${Math.abs(p.y - drag.y)}px` });
    };
    const up = (event: PointerEvent) => {
      if (!drag || drag.pointer !== event.pointerId) return;
      const p = local(event), current = latest.current;
      if (current.tool === "box" && drag.distance > 5) {
        pass(); idPicker.read({ x: Math.min(drag.x, p.x), y: Math.min(drag.y, p.y), width: Math.abs(p.x - drag.x), height: Math.abs(p.y - drag.y) }).forEach((id) => drag?.ids.add(id));
      } else if (current.tool !== "brush" && drag.distance < 6) {
        pass(); idPicker.read({ x: p.x - 1, y: p.y - 1, width: 3, height: 3 }).slice(0, 1).forEach((id) => drag?.ids.add(id));
      }
      if (drag.ids.size) current.onSelect([...drag.ids], drag.append);
      if (canvas.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId);
      drag = null; hideOverlay(); invalidate();
    };
    const cancel = () => { if (drag) anchors.current.splice(drag.anchorStart); drag = null; hideOverlay(); invalidate(); };
    const leave = () => { if (!drag) hideOverlay(); latest.current.onHover(null); };
    canvas.addEventListener("pointerdown", down); canvas.addEventListener("pointermove", move); canvas.addEventListener("pointerup", up); canvas.addEventListener("pointercancel", cancel); canvas.addEventListener("pointerleave", leave);
    return () => { canvas.removeEventListener("pointerdown", down); canvas.removeEventListener("pointermove", move); canvas.removeEventListener("pointerup", up); canvas.removeEventListener("pointercancel", cancel); canvas.removeEventListener("pointerleave", leave); idPicker.dispose(); picker.current = null; };
  }, [camera, gl, rig, invalidate]);

  useFrame((state, delta) => {
    const current = latest.current;
    if (current.playing && document.visibilityState !== "hidden") {
      phase.current = (phase.current + Math.min(delta, .05) * current.speed / getReferenceMotion(current.motion).duration) % 1;
      invalidate();
    }
    applyReferencePose(rig, current.motion, phase.current);
    if (controls.current?.update()) invalidate();
    if (state.clock.elapsedTime - reportedAt.current > .12) { current.onPhaseChange(phase.current); reportedAt.current = state.clock.elapsedTime; }
    if (marks.current) {
      marks.current.count = anchors.current.length;
      const s = scratch.current;
      anchors.current.forEach((anchor, index) => {
        anchor.mesh.getVertexPosition(anchor.triangle[0], s.a); anchor.mesh.getVertexPosition(anchor.triangle[1], s.b); anchor.mesh.getVertexPosition(anchor.triangle[2], s.c);
        s.p.copy(s.a).multiplyScalar(anchor.barycentric.x).addScaledVector(s.b, anchor.barycentric.y).addScaledVector(s.c, anchor.barycentric.z);
        s.m.makeTranslation(s.p.x, s.p.y, s.p.z); marks.current!.setMatrixAt(index, s.m);
      });
      marks.current.instanceMatrix.needsUpdate = true;
    }
  });

  return <>
    <hemisphereLight args={["#f6faf5", "#627668", 2.1]} />
    <directionalLight position={[-2, 3, 4]} intensity={3.8} color="#fffcf2" />
    <directionalLight position={[2, 2, -3]} intensity={2.4} color="#c9e4d3" />
    <primitive object={rig.group} dispose={null} />
    <instancedMesh ref={marks} args={[undefined, undefined, 160]} frustumCulled={false}>
      <sphereGeometry args={[.007, 8, 6]} /><meshBasicMaterial color="#286b50" />
    </instancedMesh>
    <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -.016, 0]}>
      <ringGeometry args={[.35, .353, 80]} /><meshBasicMaterial color={props.dark ? "#719680" : "#a5b9a8"} transparent opacity={.36} />
    </mesh>
  </>;
}

export default function BodyCanvasScene(props: BodyCanvasSceneProps) {
  const [rig, setRig] = useState<ReferenceRig | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [hover, setHover] = useState<BodyRegionId | null>(null);
  const [cursor, setCursor] = useState<HTMLDivElement | null>(null);
  const [box, setBox] = useState<HTMLDivElement | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    let owned: ReferenceRig | null = null;
    let surface: ReferenceAtlasSurface | null = null;
    setError(null);
    setRig(null);
    void (async () => {
      surface = await loadPinnedReferenceAtlasSurface({
        signal: controller.signal,
      });
      if (controller.signal.aborted) {
        surface.dispose();
        surface = null;
        return;
      }
      owned = buildReferenceRig(surface.group);
      surface.dispose();
      surface = null;
      setRig(owned);
    })().catch((cause: unknown) => {
      surface?.dispose();
      surface = null;
      if (!controller.signal.aborted) {
        setError(
          cause instanceof Error ? cause.message : "3D 暂时无法显示",
        );
      }
    });
    return () => {
      controller.abort();
      surface?.dispose();
      owned?.dispose();
    };
  }, [attempt]);
  const fallback = <div className="bc-scene-status" role="status"><h2>3D 暂时无法显示</h2><p>你的身体记录与对话仍然可以使用。</p><div className="bc-actions"><button className="bc-button" onClick={() => setAttempt((value) => value + 1)}>重试 3D</button><button className="bc-button bc-primary" onClick={props.onTextFallback}>按名称选择区域</button></div>{error && <details><summary>查看加载信息</summary><p>{error}</p></details>}</div>;
  if (error) return fallback;
  if (!rig) return <div className="bc-scene-status" role="status"><span className="bc-loading-orbit" aria-hidden="true" /><h2>正在准备身体视图</h2><p>精细参考模型按需加载，你也可以先查看记录。</p><button className="bc-button" onClick={props.onTextFallback}>先使用文本区域</button></div>;
  return <div className="bc-scene" data-tool={props.tool} data-viewer-state="ready" aria-label="交互式参考人体：拖动旋转，点击选择部位">
    <SceneBoundary key={attempt} fallback={fallback}>
      <Canvas frameloop="demand" dpr={[1, 1.5]} camera={{ position: [.36, .94, 3.12], fov: 33, near: .02, far: 20 }} gl={{ alpha: true, antialias: true, powerPreference: "high-performance" }} onCreated={({ gl }) => { gl.setClearColor(new Color(0, 0, 0), 0); gl.outputColorSpace = SRGBColorSpace; gl.toneMapping = ACESFilmicToneMapping; gl.toneMappingExposure = 1.22; }}>
        <SceneContents {...props} rig={rig} cursor={cursor} box={box} onHover={setHover} onContextLost={() => setError("图形上下文已中断，可重试或切换文本模式。")} />
      </Canvas>
    </SceneBoundary>
    <div ref={setCursor} className="bc-brush-cursor" aria-hidden="true" /><div ref={setBox} className="bc-selection-box" aria-hidden="true" />
    {hover && <span className="bc-hover-label">{getBodyRegionDefinition(hover).labels["zh-CN"]}</span>}
  </div>;
}
