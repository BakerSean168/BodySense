import { useEffect, type RefObject } from "react";
import {
  Raycaster,
  Triangle,
  Vector2,
  Vector3,
  type Camera,
  type WebGLRenderer,
} from "three";
import type { BodyRegionId } from "@/features/body-explorer/model/bodyRegionOntology";
import { SurfacePicker } from "./surfacePicker";
import type { ReferenceRig } from "./referenceRig";
import type { ReferenceCanvasContentProps } from "./types";

export interface SurfaceAnchor {
  mesh: ReferenceRig["meshes"][number];
  triangle: [number, number, number];
  barycentric: Vector3;
}

/** Mesh-coordinate painting and selection with full listener and picker cleanup.
 * Event callbacks deliberately read the latest scene props via a ref.
 */
export function useSurfacePainting({
  camera,
  gl,
  rig,
  invalidate,
  latest,
  anchors,
}: {
  camera: Camera;
  gl: WebGLRenderer;
  rig: ReferenceRig;
  invalidate: () => void;
  latest: RefObject<ReferenceCanvasContentProps>;
  anchors: RefObject<SurfaceAnchor[]>;
}) {
  useEffect(() => {
    const idPicker = new SurfacePicker(gl, rig);
    const canvas = gl.domElement;
    const ray = new Raycaster();
    let drag: {
      pointer: number;
      x: number;
      y: number;
      lastX: number;
      lastY: number;
      distance: number;
      ids: Set<BodyRegionId>;
      append: boolean;
      anchorStart: number;
    } | null = null;
    let lastPaint = 0;
    const local = (event: PointerEvent) => {
      const r = canvas.getBoundingClientRect();
      return { x: event.clientX - r.left, y: event.clientY - r.top, r };
    };
    const pass = () => {
      const r = canvas.getBoundingClientRect();
      rig.skeleton.update();
      idPicker.render(camera, r.width, r.height);
    };
    const hideOverlay = () => {
      if (latest.current.cursor) latest.current.cursor.style.display = "none";
      if (latest.current.box) latest.current.box.style.display = "none";
    };
    const addAnchor = (x: number, y: number) => {
      const region = idPicker.read({ x, y, width: 1, height: 1 })[0];
      const mesh = rig.meshes.find(
        (item) => item.userData.bodyRegionId === region,
      );
      if (!mesh || anchors.current.length >= 160) return;
      const r = canvas.getBoundingClientRect();
      ray.setFromCamera(
        new Vector2((x / r.width) * 2 - 1, 1 - (y / r.height) * 2),
        camera,
      );
      const hit = ray.intersectObject(mesh, false)[0];
      if (!hit?.face) return;
      const { a, b, c } = hit.face;
      const pointA = mesh.getVertexPosition(a, new Vector3()),
        pointB = mesh.getVertexPosition(b, new Vector3()),
        pointC = mesh.getVertexPosition(c, new Vector3());
      const barycentric = Triangle.getBarycoord(
        hit.point,
        pointA,
        pointB,
        pointC,
        new Vector3(),
      );
      if (barycentric)
        anchors.current.push({ mesh, triangle: [a, b, c], barycentric });
    };
    const paint = (x: number, y: number) => {
      pass();
      idPicker
        .read({ x: x - 17, y: y - 17, width: 34, height: 34 }, true)
        .forEach((id) => drag?.ids.add(id));
      addAnchor(x, y);
      invalidate();
    };
    const down = (event: PointerEvent) => {
      if (event.button !== 0 || drag) return;
      const p = local(event);
      drag = {
        pointer: event.pointerId,
        x: p.x,
        y: p.y,
        lastX: p.x,
        lastY: p.y,
        distance: 0,
        ids: new Set(),
        append: event.shiftKey,
        anchorStart: anchors.current.length,
      };
      if (latest.current.tool !== "orbit") {
        canvas.setPointerCapture(event.pointerId);
        latest.current.onGestureStart();
      }
      if (latest.current.tool === "brush") {
        paint(p.x, p.y);
        lastPaint = performance.now();
      }
    };
    const move = (event: PointerEvent) => {
      const p = local(event),
        current = latest.current;
      if (current.tool === "brush" && current.cursor)
        Object.assign(current.cursor.style, {
          display: "block",
          left: `${p.x}px`,
          top: `${p.y}px`,
        });
      if (!drag || event.pointerId !== drag.pointer) return;
      drag.distance += Math.abs(p.x - drag.lastX) + Math.abs(p.y - drag.lastY);
      drag.lastX = p.x;
      drag.lastY = p.y;
      if (current.tool === "brush" && performance.now() - lastPaint > 30) {
        paint(p.x, p.y);
        lastPaint = performance.now();
      }
      if (current.tool === "box" && current.box)
        Object.assign(current.box.style, {
          display: "block",
          left: `${Math.min(p.x, drag.x)}px`,
          top: `${Math.min(p.y, drag.y)}px`,
          width: `${Math.abs(p.x - drag.x)}px`,
          height: `${Math.abs(p.y - drag.y)}px`,
        });
    };
    const up = (event: PointerEvent) => {
      if (!drag || drag.pointer !== event.pointerId) return;
      const p = local(event),
        current = latest.current;
      if (current.tool === "box" && drag.distance > 5) {
        pass();
        idPicker
          .read({
            x: Math.min(drag.x, p.x),
            y: Math.min(drag.y, p.y),
            width: Math.abs(p.x - drag.x),
            height: Math.abs(p.y - drag.y),
          })
          .forEach((id) => drag?.ids.add(id));
      } else if (current.tool !== "brush" && drag.distance < 6) {
        pass();
        idPicker
          .read({ x: p.x - 1, y: p.y - 1, width: 3, height: 3 })
          .slice(0, 1)
          .forEach((id) => drag?.ids.add(id));
      }
      if (drag.ids.size) current.onSelect([...drag.ids], drag.append);
      if (canvas.hasPointerCapture(event.pointerId))
        canvas.releasePointerCapture(event.pointerId);
      drag = null;
      hideOverlay();
      invalidate();
    };
    const cancel = () => {
      if (drag) {
        anchors.current.splice(drag.anchorStart);
        if (canvas.hasPointerCapture(drag.pointer))
          canvas.releasePointerCapture(drag.pointer);
      }
      drag = null;
      hideOverlay();
      invalidate();
    };
    const leave = () => {
      // Orbit drags do not capture the pointer; pointerup may happen outside.
      if (drag && latest.current.tool === "orbit") cancel();
      if (!drag) hideOverlay();
      latest.current.onHover(null);
    };
    canvas.addEventListener("pointerdown", down);
    canvas.addEventListener("pointermove", move);
    canvas.addEventListener("pointerup", up);
    canvas.addEventListener("pointercancel", cancel);
    canvas.addEventListener("pointerleave", leave);
    return () => {
      canvas.removeEventListener("pointerdown", down);
      canvas.removeEventListener("pointermove", move);
      canvas.removeEventListener("pointerup", up);
      canvas.removeEventListener("pointercancel", cancel);
      canvas.removeEventListener("pointerleave", leave);
      cancel();
      idPicker.dispose();
    };
  }, [camera, gl, rig, invalidate]);
}
