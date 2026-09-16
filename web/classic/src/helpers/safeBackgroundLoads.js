import { loadChannelModels as preloadChannelModels } from './api';
import { classifyApiError } from './apiFailurePolicy';

// The channel page starts this optional cache warmup without awaiting it.
// Its failed request is already handled by the shared Axios availability
// manager; do not create an additional unhandled rejection in the browser.
export async function loadChannelModels() {
  try {
    return await preloadChannelModels();
  } catch (error) {
    if (classifyApiError(error)) return;
    throw error;
  }
}
