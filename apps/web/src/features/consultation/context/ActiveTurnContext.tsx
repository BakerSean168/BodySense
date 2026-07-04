/**
 * ActiveTurnContext — provides the current assistant turn's streaming state
 * and dispatch actions to the component tree.
 *
 * Uses a split-context pattern (state + actions) to avoid full-tree re-renders
 * when only the state changes.
 */

import {
  createContext,
  useContext,
  useReducer,
  useCallback,
  useRef,
  type ReactNode,
} from 'react';
import type { StreamEvent } from '../types/consultation';
import {
  reduceActiveTurnEvent,
  INITIAL_ACTIVE_TURN_STATE,
  resetActiveTurnState,
  type ActiveTurnState,
} from '../runtime/activeTurnReducer';

// ---------------------------------------------------------------------------
// Context types
// ---------------------------------------------------------------------------

interface ActiveTurnActions {
  /** Dispatch a StreamEvent through the reducer. Effects are NOT forwarded — use HYDRATE_TURN for full reducer results. */
  dispatchEvent: (event: StreamEvent) => void;
  /** Reset the active turn to initial state. */
  resetTurn: () => void;
  /** Replace the entire active turn state (e.g., hydration from history). */
  hydrateTurn: (state: ActiveTurnState) => void;
  /** Dismiss the current red flag banner. */
  dismissRedFlag: () => void;
  /** Mark a pending interaction as answered while waiting for the next run to start. */
  markInteractionAnswered: (interactionId: string) => void;
}

const ActiveTurnStateContext = createContext<ActiveTurnState>(
  INITIAL_ACTIVE_TURN_STATE,
);
const ActiveTurnActionsContext = createContext<ActiveTurnActions | null>(null);

// ---------------------------------------------------------------------------
// Reducer action types
// ---------------------------------------------------------------------------

type TurnAction =
  | { type: 'DISPATCH_EVENT'; event: StreamEvent }
  | { type: 'RESET_TURN' }
  | { type: 'HYDRATE_TURN'; state: ActiveTurnState }
  | { type: 'DISMISS_RED_FLAG' }
  | { type: 'MARK_INTERACTION_ANSWERED'; interactionId: string };

function turnReducer(state: ActiveTurnState, action: TurnAction): ActiveTurnState {
  switch (action.type) {
    case 'DISPATCH_EVENT':
      return reduceActiveTurnEvent(state, action.event).state;
    case 'RESET_TURN':
      return resetActiveTurnState();
    case 'HYDRATE_TURN':
      return action.state;
    case 'DISMISS_RED_FLAG':
      return { ...state, redFlag: null };
    case 'MARK_INTERACTION_ANSWERED':
      if (!state.pendingInteraction || state.pendingInteraction.id !== action.interactionId) {
        return state;
      }
      return {
        ...state,
        pendingInteraction: {
          ...state.pendingInteraction,
          status: 'answered',
        },
      };
  }
}

// ---------------------------------------------------------------------------
// Provider
// ---------------------------------------------------------------------------

export function ActiveTurnProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(turnReducer, INITIAL_ACTIVE_TURN_STATE);

  const dispatchEvent = useCallback((event: StreamEvent) => {
    dispatch({ type: 'DISPATCH_EVENT', event });
  }, []);

  const resetTurn = useCallback(() => {
    dispatch({ type: 'RESET_TURN' });
  }, []);

  const hydrateTurn = useCallback((s: ActiveTurnState) => {
    dispatch({ type: 'HYDRATE_TURN', state: s });
  }, []);

  const dismissRedFlag = useCallback(() => {
    dispatch({ type: 'DISMISS_RED_FLAG' });
  }, []);

  const markInteractionAnswered = useCallback((interactionId: string) => {
    dispatch({ type: 'MARK_INTERACTION_ANSWERED', interactionId });
  }, []);

  // Stable ref — functions are stable (useCallback with [] deps), so the object
  // content never changes. Keeping it in a ref avoids context value thrashing.
  const actions = useRef<ActiveTurnActions>({
    dispatchEvent,
    resetTurn,
    hydrateTurn,
    dismissRedFlag,
    markInteractionAnswered,
  });

  return (
    <ActiveTurnActionsContext.Provider value={actions.current}>
      <ActiveTurnStateContext.Provider value={state}>
        {children}
      </ActiveTurnStateContext.Provider>
    </ActiveTurnActionsContext.Provider>
  );
}

// ---------------------------------------------------------------------------
// Hooks
// ---------------------------------------------------------------------------

/** Read the current active turn state. Re-renders on every state change. */
export function useActiveTurnState(): ActiveTurnState {
  return useContext(ActiveTurnStateContext);
}

/** Get stable dispatch/reset/hydrate callbacks. Does NOT re-render on state changes. */
export function useActiveTurnActions(): ActiveTurnActions {
  const ctx = useContext(ActiveTurnActionsContext);
  if (!ctx) {
    throw new Error('useActiveTurnActions must be used within ActiveTurnProvider');
  }
  return ctx;
}
