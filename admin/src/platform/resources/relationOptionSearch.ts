export type RelationOptionSearchScheduler = {
  invalidateAll: () => void;
  isCurrent: (key: string, generation: number) => boolean;
  schedule: (key: string, task: (generation: number) => void) => number;
};

type RelationOptionSearchSchedulerOptions = {
  delayMs: number;
  setTimer: (task: () => void, delayMs: number) => number;
  clearTimer: (timer: number) => void;
};

export function createRelationOptionSearchScheduler({
  delayMs,
  setTimer,
  clearTimer,
}: RelationOptionSearchSchedulerOptions): RelationOptionSearchScheduler {
  const currentGenerations = new Map<string, number>();
  const timers = new Map<string, number>();
  let generation = 0;

  const isCurrent = (key: string, candidate: number) => currentGenerations.get(key) === candidate;

  return {
    invalidateAll() {
      for (const timer of timers.values()) {
        clearTimer(timer);
      }
      timers.clear();
      currentGenerations.clear();
    },
    isCurrent,
    schedule(key, task) {
      const pendingTimer = timers.get(key);
      if (pendingTimer !== undefined) {
        clearTimer(pendingTimer);
      }

      const nextGeneration = ++generation;
      currentGenerations.set(key, nextGeneration);
      const timer = setTimer(() => {
        if (!isCurrent(key, nextGeneration)) return;
        timers.delete(key);
        task(nextGeneration);
      }, delayMs);
      timers.set(key, timer);
      return nextGeneration;
    },
  };
}
