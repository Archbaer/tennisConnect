export const SWIPE_SLOP_PX = 10;
export const SWIPE_REVEAL_PX = 24;
export const SWIPE_OPEN_PX = 72;
export const SWIPE_MAX_PX = 110;

export type SwipeAxis = 'none' | 'horizontal' | 'vertical';
export type SwipeOutcome = 'open' | 'reveal' | 'reset';

export type SwipeRowOptions = {
  onOpen: () => void;
};

/** Decide whether a pointer gesture is a horizontal swipe or vertical scrolling. */
export function swipeAxis(dx: number, dy: number): SwipeAxis {
  if (Math.abs(dx) < SWIPE_SLOP_PX && Math.abs(dy) < SWIPE_SLOP_PX) return 'none';
  return Math.abs(dx) > Math.abs(dy) ? 'horizontal' : 'vertical';
}

/** Past the open threshold the gesture opens the row; a smaller drag only reveals the action. */
export function swipeOutcome(dx: number): SwipeOutcome {
  const distance = Math.abs(dx);
  if (distance >= SWIPE_OPEN_PX) return 'open';
  if (distance >= SWIPE_REVEAL_PX) return 'reveal';
  return 'reset';
}

/** Follow the finger with resistance after the reveal width, without runaway movement. */
export function swipeOffset(dx: number): number {
  const distance = Math.abs(dx);
  const eased = distance <= SWIPE_MAX_PX ? distance : SWIPE_MAX_PX + (distance - SWIPE_MAX_PX) * 0.3;
  return Math.sign(dx) * Math.min(eased, SWIPE_MAX_PX + 24);
}

/**
 * Pointer-driven swipe for mobile match rows. The row keeps its normal links and buttons;
 * a horizontal drag only previews "Open match" and opens past the threshold.
 */
export function swipeRow(node: HTMLElement, options: SwipeRowOptions) {
  let startX = 0;
  let startY = 0;
  let dx = 0;
  let dy = 0;
  let pointerId = -1;
  let tracking = false;
  let suppressClick = false;
  let clearClick: number | undefined;
  let axis: SwipeAxis = 'none';

  function resetStyles() {
    node.classList.remove('is-swiping', 'is-revealed', 'is-armed');
    node.style.setProperty('--swipe-offset', '0px');
  }

  function onPointerDown(event: PointerEvent) {
    if (!window.matchMedia('(max-width: 767px)').matches) return;
    if (event.pointerType === 'mouse' && event.button !== 0) return;
    if (tracking) return;
    tracking = true;
    pointerId = event.pointerId;
    axis = 'none';
    startX = event.clientX;
    startY = event.clientY;
    dx = 0;
    dy = 0;
    if (clearClick !== undefined) window.clearTimeout(clearClick);
    suppressClick = false;
  }

  function onPointerMove(event: PointerEvent) {
    if (!tracking || event.pointerId !== pointerId) return;
    dx = event.clientX - startX;
    dy = event.clientY - startY;
    if (axis === 'none') {
      axis = swipeAxis(dx, dy);
      if (axis === 'vertical') {
        tracking = false;
        return;
      }
      if (axis === 'horizontal') {
        node.classList.add('is-swiping');
        try {
          node.setPointerCapture(event.pointerId);
        } catch {
          // Synthetic pointers in tests have no capture target; the row still tracks.
        }
      }
    }
    if (axis !== 'horizontal') return;
    if (event.cancelable) event.preventDefault();
    node.style.setProperty('--swipe-offset', `${swipeOffset(dx)}px`);
    const outcome = swipeOutcome(dx);
    node.classList.toggle('is-revealed', outcome === 'reveal');
    node.classList.toggle('is-armed', outcome === 'open');
  }

  function finish(event: PointerEvent) {
    if (event.pointerId !== pointerId) return;
    const wasHorizontal = axis === 'horizontal';
    const outcome = wasHorizontal ? swipeOutcome(dx) : 'reset';
    tracking = false;
    pointerId = -1;
    axis = 'none';
    resetStyles();
    if (wasHorizontal && Math.abs(dx) > SWIPE_SLOP_PX) {
      suppressClick = true;
      clearClick = window.setTimeout(() => {
        suppressClick = false;
      }, 350);
    }
    if (outcome === 'open') options.onOpen();
  }

  function onPointerCancel(event: PointerEvent) {
    if (event.pointerId !== pointerId) return;
    tracking = false;
    pointerId = -1;
    axis = 'none';
    resetStyles();
  }

  function onClick(event: MouseEvent) {
    if (!suppressClick) return;
    suppressClick = false;
    event.preventDefault();
    event.stopPropagation();
  }

  node.addEventListener('pointerdown', onPointerDown);
  node.addEventListener('pointermove', onPointerMove);
  node.addEventListener('pointerup', finish);
  node.addEventListener('pointercancel', onPointerCancel);
  node.addEventListener('click', onClick, true);

  return {
    update(next: SwipeRowOptions) {
      options = next;
    },
    destroy() {
      if (clearClick !== undefined) window.clearTimeout(clearClick);
      node.removeEventListener('pointerdown', onPointerDown);
      node.removeEventListener('pointermove', onPointerMove);
      node.removeEventListener('pointerup', finish);
      node.removeEventListener('pointercancel', onPointerCancel);
      node.removeEventListener('click', onClick, true);
      node.style.removeProperty('--swipe-offset');
    }
  };
}
