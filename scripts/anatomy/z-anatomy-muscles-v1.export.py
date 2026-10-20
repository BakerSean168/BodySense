# Blender headless: export Z-Anatomy (CC BY-SA 4.0) visible muscles + tendons to a flat GLB.
# Output space = glTF Y-up of Z-Anatomy world (identical to Vanatome 1.4.0 / BodySense rig space).
# Produces apps/web/public/anatomy/z-anatomy-muscles-v1.glb (see THIRD_PARTY_NOTICES.md).
# Usage: blender -b <Z-Anatomy .blend> --python z-anatomy-muscles-v1.export.py -- <out.glb> <target verts>
import bpy, bmesh, re, sys, os
OUT = sys.argv[sys.argv.index('--') + 1]
TARGET_VERTS = int(sys.argv[sys.argv.index('--') + 2])
log = open(OUT + '.log', 'w')

EXCLUDE_COLLS = {'Bursae', 'Tendon sheaths', 'Laryngeal muscles', 'Pharyngeal muscles', 'Transversospinal muscles',
                 'Rotatores', 'Suboccipital muscles', 'Digestive system', 'Oculomotor nerve (III)'}
EXCLUDE_NAME = re.compile(r'bulbospongios|ischiocavernos|cremaster|sphincter|levator ani|coccygeus|perine|urethr|dartos|'
                          r'glossus|palat|uvula|tympani|stapedius|diaphragm|intercostal|subcostal|transversus thoracis|'
                          r'levatores|interspinal|intertransvers|multifid|rotatores|of eye|oblique muscle of eye|'
                          r'levator palpebrae|tarsal|ciliary|arytenoid|cricothyroid|tensor veli|capsule|bursa|sheath|'
                          r'synovial|mesotendon|vinculum|transversus abdominis|internal abdominal oblique|psoas minor|'
                          r'quadratus lumborum|piriformis|obturator internus|gemellus|iliacus|psoas major|pyramidalis|anal|rectus muscle\.|'
 r'oblique muscle\.|trochlea|pterygoid|longus capitis|longus colli|rectus anterior capitis|rectus lateralis capitis|'
 r'geniohyoid|mylohyoid|stylohyoid|digastric|subclavius|vastus intermedius|tibialis posterior|popliteus|'
 r'gluteus minimus|obturator externus|adductor minimus|interossei|lumbrical|quadratus plantae|supinator|'
 r'pronator quadratus|flexor digitorum profundus|flexor pollicis longus|subscapularis|deep part of masseter|serratus posterior', re.I)
TENDON_NAME = re.compile(r'tendon|aponeurosis|tract|retinacul|linea alba|ligament|tendinous|galea', re.I)
FASCIA_KEEP = re.compile(r'iliotibial tract|plantar aponeurosis|palmar aponeurosis|epicranial aponeurosis', re.I)

def suffix(o):
    m = re.search(r'\.([a-z]+)$', o.name)
    return m.group(1) if m else ''

ms = bpy.data.collections['4: Muscular system']
sup = set(o.name for o in bpy.data.collections['Superficial muscles'].all_objects)
mus = set(o.name for o in bpy.data.collections['Muscles'].all_objects)
cands = []
for o in ms.all_objects:
    if o.type != 'MESH' or suffix(o) not in ('l', 'r', '') or not o.data.polygons:
        continue
    colls = {c.name for c in o.users_collection}
    if colls & EXCLUDE_COLLS or EXCLUDE_NAME.search(o.name):
        continue
    if 'Fascia' in colls or 'Fascia of upper limb' in colls or re.search(r'fascia', o.name, re.I):
        if not FASCIA_KEEP.search(o.name):
            continue
    if o.name in mus or o.name in sup or FASCIA_KEEP.search(o.name) or TENDON_NAME.search(o.name):
        cands.append(o)

for o in cands:
    for m in o.modifiers:
        if m.type in ('SUBSURF', 'WIREFRAME', 'MULTIRES'):
            m.show_viewport = False
            m.show_render = False
deps = bpy.context.evaluated_depsgraph_get()
deps.update()

coll = bpy.data.collections.new('EXPORT')
bpy.context.scene.collection.children.link(coll)
made = []
total_src = 0
for o in cands:
    eo = o.evaluated_get(deps)
    me = eo.to_mesh()
    bm = bmesh.new()
    bm.from_mesh(me)
    eo.to_mesh_clear()
    bm.transform(o.matrix_world)
    if o.matrix_world.determinant() < 0:
        bmesh.ops.reverse_faces(bm, faces=bm.faces)
    slots = [(s.material.name if s.material else '') for s in o.material_slots]
    whole_tendon = bool(TENDON_NAME.search(o.name))
    for tissue in ('muscle', 'tendon'):
        b2 = bm.copy()
        kill = []
        for f in b2.faces:
            mat = slots[f.material_index] if f.material_index < len(slots) else ''
            is_t = whole_tendon or mat in ('Tendon', 'Ligament')
            if is_t != (tissue == 'tendon'):
                kill.append(f)
        bmesh.ops.delete(b2, geom=kill, context='FACES')
        if len(b2.faces) == 0:
            b2.free(); continue
        bmesh.ops.triangulate(b2, faces=b2.faces)
        mesh = bpy.data.meshes.new(o.name + '__' + tissue)
        b2.to_mesh(mesh); b2.free()
        mesh.materials.clear()
        ob = bpy.data.objects.new(o.name + '__' + tissue, mesh)
        coll.objects.link(ob)
        made.append(ob)
        total_src += len(mesh.vertices)
    bm.free()

ratio = min(1.0, TARGET_VERTS / max(1, total_src))
log.write(f'cands={len(cands)} parts={len(made)} srcverts={total_src} ratio={ratio:.3f}\n')
for ob in made:
    n = len(ob.data.vertices)
    r = max(ratio, min(1.0, 120 / max(1, n)))
    if r < 0.999:
        d = ob.modifiers.new('dec', 'DECIMATE'); d.ratio = r; d.use_collapse_triangulate = True
    for p in ob.data.polygons: p.use_smooth = True

for o in bpy.context.view_layer.objects:
    o.select_set(False)
for ob in made:
    ob.select_set(True)
bpy.context.view_layer.objects.active = made[0]
bpy.ops.export_scene.gltf(filepath=OUT, export_format='GLB', use_selection=True, export_apply=True, export_yup=True,
                          export_materials='NONE', export_texcoords=False, export_normals=True,
                          export_attributes=False, export_skins=False, export_morph=False, export_animations=False,
                          export_draco_mesh_compression_enable=False, export_extras=False, export_cameras=False,
                          export_lights=False)
log.write(f'written {OUT} {os.path.getsize(OUT)}\n')
for o in cands:
    log.write('C ' + o.name + '\n')
log.close()
