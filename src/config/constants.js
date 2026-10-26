/**
 * Game Configuration Constants
 */

export const GAME_VERSION = '1.0.0';

export const API_BASE_URL = '/api';

/** The 7-day week (types.WeekDays on the server); day 1 is Firstday. */
export const WEEK_DAYS = ['Firstday', 'Toilday', 'Midweek', 'Hearthday', 'Fairday', 'Marketday', 'Restday'];

/** The day of the week of a game day number. */
export function weekdayOf(day) {
    return WEEK_DAYS[(((Number(day) || 1) - 1) % 7 + 7) % 7];
}
