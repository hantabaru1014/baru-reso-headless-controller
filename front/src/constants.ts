export const AccessLevels = [
  { id: "1", labelKey: "constants.accessLevels.private", value: 1 },
  { id: "2", labelKey: "constants.accessLevels.lan", value: 2 },
  { id: "3", labelKey: "constants.accessLevels.contacts", value: 3 },
  { id: "4", labelKey: "constants.accessLevels.contactsPlus", value: 4 },
  { id: "5", labelKey: "constants.accessLevels.registeredUsers", value: 5 },
  { id: "6", labelKey: "constants.accessLevels.anyone", value: 6 },
] as const;

export const UserRoles = [
  { id: "Admin", label: "Admin" },
  { id: "Builder", label: "Builder" },
  { id: "Moderator", label: "Moderator" },
  { id: "Guest", label: "Guest" },
  { id: "Spectator", label: "Spectator" },
] as const;
