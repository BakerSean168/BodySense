import type { Group } from "three";

export interface ReferenceAtlasSurface {
  group: Group;
  modelUrl: string;
  dispose: () => void;
}
