import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const ledgerPath = path.join(root, 'docs/learning/curriculum/ledger/full-stack-open.json');
const currentPath = path.join(root, 'docs/learning/curriculum/ledger/full-stack-open-current-mooc.json');
const ledger = JSON.parse(fs.readFileSync(ledgerPath, 'utf8'));
const current = JSON.parse(fs.readFileSync(currentPath, 'utf8'));

const isHistoricalAdvancedExercise = (item) =>
  item.kind === 'exercise' &&
  item.source?.part >= 8 && item.source?.part <= 11 &&
  item.source?.snapshot_commit === ledger.baseline?.indexed_snapshot_commit;

const historicalItems = [
  ...(ledger.historical_items ?? []),
  ...ledger.items.filter(isHistoricalAdvancedExercise),
];
const historicalById = new Map(historicalItems.map((item) => [item.id, item]));
ledger.historical_items = [...historicalById.values()].sort((a, b) => {
  if (a.source.part !== b.source.part) return a.source.part - b.source.part;
  return String(a.source.number ?? a.id).localeCompare(String(b.source.number ?? b.id), undefined, { numeric: true });
});

const currentExisting = new Map(
  ledger.items
    .filter((item) => item.source?.authority === 'current-mooc-api' && item.source?.platform_exercise_id)
    .map((item) => [item.source.platform_exercise_id, item]),
);

ledger.items = ledger.items.filter((item) => {
  if (isHistoricalAdvancedExercise(item)) return false;
  if (item.kind === 'source_boundary' && item.source?.part >= 12 && item.source?.part <= 14) return false;
  if (item.source?.authority === 'current-mooc-api' && item.source?.part >= 8 && item.source?.part <= 14) return false;
  return true;
});

const currentItems = [];
for (const course of current.courses) {
  for (const exercise of course.exercises) {
    const existing = currentExisting.get(exercise.id);
    const id = existing?.id ?? `FSO-P${course.part}-MOOC-${exercise.id}`;
    currentItems.push({
      id,
      kind: 'exercise',
      source: {
        course: 'full-stack-open',
        part: course.part,
        platform: 'courses.mooc.fi',
        platform_exercise_id: exercise.id,
        title: exercise.name,
        course_slug: course.slug,
        course_id: course.course_id,
        course_url: course.course_url,
        chapter_id: exercise.chapter_id,
        chapter_number: exercise.chapter_number,
        chapter_name: exercise.chapter_name,
        page_id: exercise.page_id,
        page_path: exercise.page_path,
        page_title: exercise.page_title,
        updated_at: exercise.updated_at,
        verification: 'VERIFIED_CURRENT_MOOC_API_INDEX',
        authority: 'current-mooc-api',
        source_snapshot_sha256: current.source_state_sha256,
      },
      lifecycle: existing?.lifecycle ?? 'SOURCE_INDEXED',
      mapping: existing?.mapping ?? null,
      mastery: existing?.mastery ?? null,
    });
  }
}
currentItems.sort((a, b) => {
  if (a.source.part !== b.source.part) return a.source.part - b.source.part;
  const ca = a.source.chapter_number ?? Number.MAX_SAFE_INTEGER;
  const cb = b.source.chapter_number ?? Number.MAX_SAFE_INTEGER;
  if (ca !== cb) return ca - cb;
  return (a.source.page_path ?? '').localeCompare(b.source.page_path ?? '') || a.id.localeCompare(b.id);
});
ledger.items.push(...currentItems);

// Keep only active-source section headings for Parts 0-7 in source_sections.
const previousSections = ledger.source_sections ?? [];
ledger.historical_source_sections = [
  ...(ledger.historical_source_sections ?? []),
  ...previousSections.filter((item) => item.source?.part >= 8 && item.source?.part <= 11),
];
const historicalSectionById = new Map(ledger.historical_source_sections.map((item) => [item.id, item]));
ledger.historical_source_sections = [...historicalSectionById.values()];
ledger.source_sections = previousSections.filter((item) => item.source?.part <= 7);

ledger.current_mooc_sections = current.courses.flatMap((course) =>
  course.headings.map((heading, index) => ({
    id: `FSO-P${course.part}-MOOC-SECTION-${String(index + 1).padStart(3, '0')}`,
    kind: 'source_section',
    source: {
      course: 'full-stack-open',
      part: course.part,
      platform: 'courses.mooc.fi',
      course_slug: course.slug,
      course_id: course.course_id,
      course_url: course.course_url,
      chapter_id: heading.chapter_id,
      page_id: heading.page_id,
      page_path: heading.page_path,
      page_title: heading.page_title,
      heading_level: heading.level,
      title: heading.text,
      verification: 'VERIFIED_CURRENT_MOOC_API_INDEX',
      authority: 'current-mooc-api',
      source_snapshot_sha256: current.source_state_sha256,
    },
    lifecycle: 'SOURCE_INDEXED',
    mapping: null,
    mastery: null,
  })),
);

ledger.baseline.current_mooc = {
  retrieved_at: current.retrieved_at,
  source_state_sha256: current.source_state_sha256,
  parts: current.courses.map((course) => ({
    part: course.part,
    slug: course.slug,
    course_id: course.course_id,
    front_page_updated_at: course.front_page.updated_at,
    exercise_records: course.counts.exercise_records,
    headings: course.counts.headings,
  })),
  authority: 'Metadata indexed directly from the public courses.mooc.fi Course Material API; semantic mapping remains separate.',
};
ledger.baseline.current_parity_scope = 'Parts 0-7 exercise semantics mapped from pinned course-repository snapshot. Parts 8-14 current MOOC exercise/heading metadata are source-indexed from the public Course Material API; semantic mapping/readiness remain pending.';

fs.writeFileSync(ledgerPath, `${JSON.stringify(ledger, null, 2)}\n`);
console.log(`synced ${currentItems.length} current MOOC exercise records into ${path.relative(root, ledgerPath)}`);
