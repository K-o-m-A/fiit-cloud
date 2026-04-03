package compose.project.mudrodnabe.entities;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import org.springframework.data.annotation.Id;
import org.springframework.data.mongodb.core.mapping.Document;

@Data
@Document(collection = "quotes")
@AllArgsConstructor
@NoArgsConstructor
public class Quote {
    @Id private String id;
    private String quote;
}
