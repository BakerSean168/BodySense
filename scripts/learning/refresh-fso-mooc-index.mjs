import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const outPath = path.join(root, 'docs/learning/curriculum/ledger/full-stack-open-current-mooc.json');
const apiBase = 'https://courses.mooc.fi/api/v0/course-material';

const courses = [
  { part: 8, slug: 'full-stack-open-graphql', label: 'GraphQL' },
  { part: 9, slug: 'full-stack-open-typescript', label: 'TypeScript' },
  { part: 10, slug: 'full-stack-open-react-native', label: 'React Native' },
  { part: 11, slug: 'full-stack-open-continuous-integration', label: 'Continuous Integration' },
  { part: 12, slug: 'full-stack-open-containers', label: 'Containers' },
  { part: 13, slug: 'full-stack-open-relational-databases', label: 'Relational Databases' },
  { part: 14, slug: 'full-stack-open-nextjs', label: 'Next.js' },
];

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function getJson(url, attempt = 0) {
  const response = await fetch(url, {
    headers: {
      accept: 'application/json',
      'user-agent': 'BodySense-curriculum-source-index/1.0',
    },
  });
  if (response.status === 429 && attempt < 7) {
    const retryAfter = Number(response.headers.get('retry-after') ?? 0);
    const delay = retryAfter > 0 ? retryAfter * 1000 : Math.min(15000, 1000 * 2 ** attempt);
    await sleep(delay);
    return getJson(url, attempt + 1);
  }
  if (!response.ok) {
    throw new Error(`${response.status} ${response.statusText}: ${url}`);
  }
  return response.json();
}

async function mapLimit(values, limit, fn) {
  const results = new Array(values.length);
  let cursor = 0;
  async function worker() {
    while (true) {
      const index = cursor++;
      if (index >= values.length) return;
      results[index] = await fn(values[index], index);
      await sleep(125);
    }
  }
  await Promise.all(Array.from({ length: Math.min(limit, values.length) }, worker));
  return results;
}

function* walkBlocks(block) {
  yield block;
  for (const child of block?.innerBlocks ?? []) yield* walkBlocks(child);
}

function stripHtml(value) {
  return String(value ?? '')
    .replace(/<[^>]+>/g, '')
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/\s+/g, ' ')
    .trim();
}

function stableStringify(value) {
  if (Array.isArray(value)) return `[${value.map(stableStringify).join(',')}]`;
  if (value && typeof value === 'object') {
    return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${stableStringify(value[key])}`).join(',')}}`;
  }
  return JSON.stringify(value);
}

async function indexCourse(courseDef) {
  const front = await getJson(`${apiBase}/courses/${courseDef.slug}/page-by-path/`);
  const frontPage = front.page;
  const courseId = frontPage.course_id;
  const [pagesRaw, chaptersRaw] = await Promise.all([
    getJson(`${apiBase}/courses/${courseId}/pages`),
    getJson(`${apiBase}/courses/${courseId}/chapters`),
  ]);

  const chapters = (chaptersRaw.modules ?? [])
    .flatMap((module) => module.chapters ?? [])
    .map((chapter) => ({
      id: chapter.id,
      name: chapter.name,
      chapter_number: chapter.chapter_number,
      front_page_id: chapter.front_page_id,
      updated_at: chapter.updated_at,
      status: chapter.status,
    }))
    .sort((a, b) => a.chapter_number - b.chapter_number);
  const chapterById = new Map(chapters.map((chapter) => [chapter.id, chapter]));

  const pages = pagesRaw.map((page) => ({
    id: page.id,
    title: page.title,
    url_path: page.url_path,
    chapter_id: page.chapter_id,
    updated_at: page.updated_at,
  }));
  const pageById = new Map(pages.map((page) => [page.id, page]));

  const exerciseIds = [];
  const headings = [];
  for (const page of pagesRaw) {
    for (const topBlock of page.content ?? []) {
      for (const block of walkBlocks(topBlock)) {
        if (block?.name === 'moocfi/exercise' && block.attributes?.id) {
          exerciseIds.push(block.attributes.id);
        }
        if (block?.name === 'core/heading') {
          const text = stripHtml(block.attributes?.content);
          if (text) {
            headings.push({
              page_id: page.id,
              page_path: page.url_path,
              page_title: page.title,
              chapter_id: page.chapter_id,
              level: block.attributes?.level ?? null,
              text,
            });
          }
        }
      }
    }
  }

  const uniqueExerciseIds = [...new Set(exerciseIds)];
  const exerciseRows = await mapLimit(
    uniqueExerciseIds,
    3,
    async (exerciseId) => {
      const detail = await getJson(`${apiBase}/exercises/${exerciseId}`);
      const exercise = detail.exercise;
      const page = pageById.get(exercise.page_id) ?? null;
      const chapter = chapterById.get(exercise.chapter_id) ?? null;
      const tasks = detail.current_exercise_slide?.exercise_tasks ?? [];
      return {
        id: exercise.id,
        name: exercise.name ?? '',
        created_at: exercise.created_at,
        updated_at: exercise.updated_at,
        score_maximum: exercise.score_maximum,
        order_number: exercise.order_number,
        page_id: exercise.page_id,
        page_path: page?.url_path ?? null,
        page_title: page?.title ?? null,
        chapter_id: exercise.chapter_id,
        chapter_number: chapter?.chapter_number ?? null,
        chapter_name: chapter?.name ?? null,
        task_count: tasks.length,
        exercise_service_slugs: [...new Set(tasks.map((task) => task.exercise_service_slug).filter(Boolean))].sort(),
      };
    },
  );

  exerciseRows.sort((a, b) => {
    const ca = a.chapter_number ?? Number.MAX_SAFE_INTEGER;
    const cb = b.chapter_number ?? Number.MAX_SAFE_INTEGER;
    if (ca !== cb) return ca - cb;
    if ((a.page_path ?? '') !== (b.page_path ?? '')) return (a.page_path ?? '').localeCompare(b.page_path ?? '');
    if (a.order_number !== b.order_number) return a.order_number - b.order_number;
    return a.id.localeCompare(b.id);
  });

  return {
    part: courseDef.part,
    label: courseDef.label,
    slug: courseDef.slug,
    course_id: courseId,
    course_url: `https://courses.mooc.fi/org/uh-cs/courses/${courseDef.slug}`,
    api_front_page_url: `${apiBase}/courses/${courseDef.slug}/page-by-path/`,
    front_page: {
      id: frontPage.id,
      title: frontPage.title,
      updated_at: frontPage.updated_at,
    },
    chapters,
    pages,
    headings,
    exercises: exerciseRows,
    counts: {
      chapters: chapters.length,
      pages: pages.length,
      headings: headings.length,
      exercise_records: exerciseRows.length,
    },
  };
}

const indexedCourses = [];
for (const course of courses) {
  indexedCourses.push(await indexCourse(course));
  await sleep(500);
}
indexedCourses.sort((a, b) => a.part - b.part);

const sourceState = {
  schema_version: 1,
  platform: 'courses.mooc.fi',
  organization: 'uh-cs',
  api_base: apiBase,
  courses: indexedCourses,
};
const digest = crypto.createHash('sha256').update(stableStringify(sourceState)).digest('hex');
const output = {
  ...sourceState,
  retrieved_at: new Date().toISOString(),
  source_state_sha256: digest,
  scope_note: 'Metadata-only snapshot: course/chapter/page/heading/exercise identifiers and short titles. Exercise assignments, answers and course prose are intentionally not copied.',
};

fs.mkdirSync(path.dirname(outPath), { recursive: true });
fs.writeFileSync(outPath, `${JSON.stringify(output, null, 2)}\n`);
console.log(`wrote ${path.relative(root, outPath)}`);
for (const course of indexedCourses) {
  console.log(`Part ${course.part}: ${course.counts.exercise_records} exercises, ${course.counts.headings} headings, ${course.counts.pages} pages`);
}
console.log(`source state sha256: ${digest}`);
