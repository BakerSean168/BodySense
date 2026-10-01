export interface RetryDynamicImportOptions {
  attempts?: number;
  delayMs?: number;
}

export async function retryDynamicImport<T>(
  loader: () => Promise<T>,
  { attempts = 2, delayMs = 250 }: RetryDynamicImportOptions = {},
): Promise<T> {
  if (!Number.isInteger(attempts) || attempts < 1) {
    throw new Error(`retryDynamicImport attempts must be >= 1: ${attempts}`);
  }
  if (!Number.isFinite(delayMs) || delayMs < 0) {
    throw new Error(`retryDynamicImport delayMs must be >= 0: ${delayMs}`);
  }

  let lastError: unknown;
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    try {
      return await loader();
    } catch (error) {
      lastError = error;
      if (attempt < attempts && delayMs > 0) {
        await new Promise((resolve) =>
          setTimeout(resolve, delayMs * 2 ** (attempt - 1)),
        );
      }
    }
  }

  throw lastError;
}
