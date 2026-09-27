#include <iostream>
#include <string>
#include <cmath>
// SDK supplies the checksum-pinned nlohmann/json header via -include.
using nlohmann::json;

int main() {
    std::ios_base::sync_with_stdio(false);
    std::cin.tie(nullptr);
    std::string line;
    while (std::getline(std::cin, line)) {
        if (line.size() > 1024 * 1024) return 1;
        try {
            const auto message = json::parse(line);
            const auto phase = message.at("phase").get<std::string>();
            if (phase == "INIT") {
                if (message.at("protocol_version").get<int>() != 1) return 1;
                std::cout << json({{"status", "READY"}}).dump() << '\n' << std::flush;
            } else if (phase == "TICK") {
                const auto tick = message.at("tick").get<unsigned int>();
                const auto& position = message.at("you").at("pos");
                const auto& center = message.at("zone").at("center");
                const double dx = center.at(0).get<double>() - position.at(0).get<double>();
                const double dy = center.at(1).get<double>() - position.at(1).get<double>();
                const double angle = std::atan2(dy, dx);
                const bool shoot = !message.at("visible_enemies").empty();
                std::cout << json({{"tick", tick}, {"angle", angle}, {"shoot", shoot}}).dump()
                          << '\n' << std::flush;
            } else if (phase == "TERMINATE") {
                return 0;
            }
        } catch (const json::exception& error) {
            std::cerr << "Invalid observation: " << error.what() << '\n';
            return 1;
        }
    }
    return 0;
}
