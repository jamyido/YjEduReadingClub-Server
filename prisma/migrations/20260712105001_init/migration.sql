/*
  Warnings:

  - You are about to drop the column `points` on the `check_ins` table. All the data in the column will be lost.
  - You are about to drop the column `points_reward` on the `courses` table. All the data in the column will be lost.
  - You are about to drop the column `points` on the `users` table. All the data in the column will be lost.
  - You are about to drop the column `total_points` on the `users` table. All the data in the column will be lost.
  - You are about to drop the `point_records` table. If the table is not empty, all the data it contains will be lost.

*/
-- DropForeignKey
ALTER TABLE `point_records` DROP FOREIGN KEY `point_records_user_id_fkey`;

-- AlterTable
ALTER TABLE `check_ins` DROP COLUMN `points`;

-- AlterTable
ALTER TABLE `courses` DROP COLUMN `points_reward`;

-- AlterTable
ALTER TABLE `users` DROP COLUMN `points`,
    DROP COLUMN `total_points`;

-- DropTable
DROP TABLE `point_records`;
