#include <iostream>
#include <string>
#include <cmath>

// Parseador JSON ultraligero y rápido para las observaciones mínimas
// Evita dependencias externas de librerías en C++

int main() {
    std::ios_base::sync_with_stdio(false);
    std::cin.tie(NULL);

    std::string line;
    while (std::getline(std::cin, line)) {
        if (line.empty()) continue;

        if (line.find("\"INIT\"") != std::string::npos) {
            std::cout << "{\"status\":\"READY\"}\n" << std::flush;
        } else if (line.find("\"TICK\"") != std::string::npos) {
            unsigned int tick = 0;
            size_t tick_idx = line.find("\"tick\":");
            if (tick_idx != std::string::npos) {
                sscanf(line.c_str() + tick_idx + 7, "%u", &tick);
            }
            // Extraer posición propia "pos":[x, y]
            float my_x = 600.0f, my_y = 375.0f;
            size_t pos_idx = line.find("\"pos\":[");
            if (pos_idx != std::string::npos) {
                sscanf(line.c_str() + pos_idx + 7, "%f,%f", &my_x, &my_y);
            }

            // Calcular ángulo hacia el centro del mapa (600, 375)
            float dx = 600.0f - my_x;
            float dy = 375.0f - my_y;
            float angle = std::atan2(dy, dx);

            // Disparar si detecta enemigos visibles
            bool shoot = (line.find("\"visible_enemies\":[]") == std::string::npos);

            std::cout << "{\"tick\":" << tick << ",\"angle\":" << angle << ",\"shoot\":" << (shoot ? "true" : "false") << "}\n" << std::flush;
        } else if (line.find("\"TERMINATE\"") != std::string::npos) {
            break;
        }
    }

    return 0;
}
